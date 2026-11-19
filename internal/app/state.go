package app

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"nebula_client/internal/graphview"
	"nebula_client/internal/i18n"
	"nebula_client/internal/model"
	"nebula_client/internal/nebula"
	"nebula_client/internal/ngql"
)

const demoStarterQuery = "MATCH p=(v)-[e:follow|serve|coaches]->(n)\nRETURN p\nLIMIT 26;"

// QueryTab holds the independent state of one query tab in the workspace.
type QueryTab struct {
	ID             string
	Title          string
	Editor         *editor.State
	Result         *model.QueryResult
	Graph          *graphview.State
	ResultTab      int // 0=Graph, 1=Table, 2=JSON, 3=Plan
	TableState     ui.ListState
	TableSort      ui.SortOrder
	TableFilter    string
	SelectedRow    int
	VIDColChoice   string
	JSONEditor     *editor.State
	Running        bool
	Expanding      bool
	SelectedVertex *model.Vertex
	SelectedEdge   *model.Edge

	// Cached sorted/filtered row indices to avoid re-sorting every UI frame
	cachedRowResult *model.QueryResult
	cachedRowFilter string
	cachedRowSort   ui.SortOrder
	cachedRowIdx    []int
}

// App is the top-level state for the NebulaDesk native desktop application.
type App struct {
	Win   *mygo.Window
	Store *model.ConfigStore
	Config *model.AppConfig

	// Database connection state
	Client       nebula.Client
	ActiveConn   model.ConnectionProfile
	Spaces       []string
	CurrentSpace string
	Schema       *model.SpaceSchema

	// Workspace tabs
	Tabs      []*QueryTab
	ActiveTab int
	nextTabID int

	// Layout state
	SidebarTab     int // 0=Schema, 1=History, 2=Snippets
	SidebarWidth   float32
	EditorPercent  float32
	InspectorWidth float32
	ShowSidebar    bool
	ShowInspector  bool
	ExpandLimit    int // Max paths returned per interactive node expansion (default 50)

	// Schema tree open state
	TagsSectionOpen    bool
	EdgesSectionOpen   bool
	IndexesSectionOpen bool
	TagExpanded        map[string]bool
	EdgeExpanded       map[string]bool

	// History / Snippet search
	HistorySearch string

	// Connection modal state
	ConnModalOpen bool
	ConnSelected  int
	ConnDraft     model.ConnectionProfile
	ConnPortText  string
	ConnError     string
	Connecting    bool

	// Save snippet modal state
	SaveSnippetOpen  bool
	SnippetTitle     string
	SnippetCategory  string

	// Export modal state
	ExportModalOpen bool
	ExportTitle     string
	ExportExt       string
	ExportContent   string
	ExportEditor    *editor.State

	// Status bar & notifications
	StatusMsg   string
	StatusIsErr bool
	ToastQueue  string

	// Async dispatcher (can be overridden to synchronous in unit tests)
	RunAsync func(work func() func())
}

// NewApp constructs the application state, loads persisted config, connects to the
// starter profile (Built-in Demo by default), and executes an initial exploration query.
func NewApp(store *model.ConfigStore) *App {
	if store == nil {
		store = model.NewDefaultConfigStore()
	}
	cfg := store.Load()

	a := &App{
		Store:              store,
		Config:             cfg,
		SidebarWidth:       265,
		EditorPercent:      22,
		InspectorWidth:     295,
		ShowSidebar:        true,
		ShowInspector:      true,
		ExpandLimit:        50,
		TagsSectionOpen:    true,
		EdgesSectionOpen:   true,
		IndexesSectionOpen: false,
		TagExpanded:        make(map[string]bool),
		EdgeExpanded:       make(map[string]bool),
		SnippetCategory:    "Explore",
		nextTabID:          1,
	}

	a.RunAsync = func(work func() func()) {
		go func() {
			commit := work()
			if commit == nil {
				return
			}
			if a.Win != nil {
				a.Win.Update(commit)
			} else {
				commit()
			}
		}()
	}

	// Pick initial connection profile: honor cfg.LastConnID.
	// Only auto-connect & pre-populate the built-in Demo dataset when LastConnID is a Demo profile
	// (e.g., first launch). If the user already uses a real cluster, boot cleanly with no demo data.
	initialIdx := 0
	for i, c := range cfg.Connections {
		if c.ID == cfg.LastConnID {
			initialIdx = i
			break
		}
	}
	a.ConnSelected = initialIdx
	if initialIdx < len(cfg.Connections) {
		a.ConnDraft = cfg.Connections[initialIdx]
		a.ConnPortText = strconv.Itoa(a.ConnDraft.Port)
	}

	if initialIdx < len(cfg.Connections) && cfg.Connections[initialIdx].Demo {
		a.NewTab("Query 1", demoStarterQuery)
		if client, err := nebula.Connect(cfg.Connections[initialIdx]); err == nil {
			a.attachClient(client, cfg.Connections[initialIdx])
			a.executeQuerySync(a.CurrentTab(), demoStarterQuery)
		}
	} else {
		a.NewTab("Query 1", "")
		if initialIdx < len(cfg.Connections) {
			a.setStatus("Ready • Click connection pill (⌘K) to connect to "+cfg.Connections[initialIdx].Name, false)
		}
	}

	return a
}

// CurrentTab returns the active QueryTab.
func (a *App) CurrentTab() *QueryTab {
	if len(a.Tabs) == 0 {
		a.NewTab("Query 1", "")
	}
	if a.ActiveTab < 0 || a.ActiveTab >= len(a.Tabs) {
		a.ActiveTab = 0
	}
	return a.Tabs[a.ActiveTab]
}

// NewTab creates and activates a new query tab with a configured nGQL code editor.
func (a *App) NewTab(title, initialQuery string) *QueryTab {
	if title == "" {
		title = fmt.Sprintf("Query %d", a.nextTabID)
	}
	id := fmt.Sprintf("tab-%d", a.nextTabID)
	a.nextTabID++

	buf := editor.NewBuffer(initialQuery)
	buf.SetLanguage(ngql.Language())

	ed := &editor.State{
		Buffer:      buf,
		Placeholder: "Write nGQL here… (⌘Enter / Ctrl+Enter to run, Ctrl+Space for schema autocomplete)",
		SoftWrap:    true,
		TabSize:     2,
		// Let Cmd+Enter bubble up to app shortcut for Run Query instead of inserting newline below
		Bindings: []editor.Binding{
			{Mods: ui.Cmd, Key: ui.KeyEnter, Command: editor.NoCommand},
			{Mods: ui.Ctrl, Key: ui.KeyEnter, Command: editor.NoCommand},
		},
	}

	jsonBuf := editor.NewBuffer("{}")
	jsonBuf.SetLanguage(editor.LanguageNamed("JSON"))
	jsonEd := &editor.State{
		Buffer:   jsonBuf,
		ReadOnly: true,
		SoftWrap: true,
	}

	tab := &QueryTab{
		ID:          id,
		Title:       title,
		Editor:      ed,
		Graph:       graphview.NewState(),
		JSONEditor:  jsonEd,
		SelectedRow: -1,
	}
	tab.TableState.Selected = &tab.SelectedRow
	tab.TableState.Sort = &tab.TableSort

	// Hook schema-aware autocompletion & hover docs into the Zed-style editor
	ed.OnCompletion = func(r editor.CompletionRequest) {
		items := ngql.Completions(a.Schema)
		ed.ShowCompletions(r.Offset-len(r.Prefix), items)
	}
	ed.OnHover = func(r editor.HoverRequest) {
		word, start, end := wordAtOffset(buf.String(), r.Offset)
		if doc, ok := ngql.HoverDoc(word, a.Schema); ok {
			ed.ShowHover(start, end, doc, nil)
		}
	}

	a.Tabs = append(a.Tabs, tab)
	a.ActiveTab = len(a.Tabs) - 1
	return tab
}

// CloseTab closes the tab at index idx, keeping at least one tab open.
func (a *App) CloseTab(idx int) {
	if len(a.Tabs) <= 1 || idx < 0 || idx >= len(a.Tabs) {
		return
	}
	a.Tabs = append(a.Tabs[:idx], a.Tabs[idx+1:]...)
	if a.ActiveTab >= len(a.Tabs) {
		a.ActiveTab = len(a.Tabs) - 1
	}
}

func (a *App) attachClient(client nebula.Client, profile model.ConnectionProfile) {
	if a.Client != nil {
		a.Client.Close()
	}
	a.Client = client
	a.ActiveConn = profile
	a.Config.LastConnID = profile.ID
	_ = a.Store.Save(a.Config)

	spaces, err := client.Spaces()
	if err == nil {
		a.Spaces = spaces
	} else {
		a.Spaces = nil
	}

	space := client.CurrentSpace()
	if space == "" && len(a.Spaces) > 0 {
		space = a.Spaces[0]
		_ = client.UseSpace(space)
	}
	a.CurrentSpace = space
	if space != "" {
		if sc, err := client.LoadSchema(space); err == nil {
			a.Schema = sc
		}
	} else {
		a.Schema = nil
	}
	a.setStatus(fmt.Sprintf("Connected to %s (%s)", profile.Name, profile.Address()), false)
}

// ConnectTo connects asynchronously to the given profile.
func (a *App) ConnectTo(profile model.ConnectionProfile) {
	a.Connecting = true
	a.ConnError = ""
	a.setStatus("Connecting to "+profile.Address()+"…", false)

	a.RunAsync(func() func() {
		client, err := nebula.Connect(profile)
		if err != nil {
			return func() {
				a.Connecting = false
				a.ConnError = err.Error()
				a.setStatus("Connection failed: "+err.Error(), true)
			}
		}
		spaces, _ := client.Spaces()
		space := client.CurrentSpace()
		if space == "" && len(spaces) > 0 {
			space = spaces[0]
			_ = client.UseSpace(space)
		}
		var schema *model.SpaceSchema
		if space != "" {
			schema, _ = client.LoadSchema(space)
		}
		return func() {
			if a.Client != nil {
				a.Client.Close()
			}
			a.Client = client
			a.ActiveConn = profile
			a.Spaces = spaces
			a.CurrentSpace = space
			a.Schema = schema
			a.Connecting = false
			a.ConnError = ""
			a.ConnModalOpen = false
			a.Config.LastConnID = profile.ID
			_ = a.Store.Save(a.Config)
			a.clearAllTabResults(profile.Demo)
			a.setStatus(fmt.Sprintf("Connected to %s (%s)", profile.Name, profile.Address()), false)
			a.ToastQueue = "Connected to " + profile.Name
		}
	})
}

// SwitchSpace runs USE <space> and reloads schema metadata.
func (a *App) SwitchSpace(space string) {
	if a.Client == nil || space == "" {
		return
	}
	client := a.Client
	a.setStatus("Switching space to "+space+"…", false)
	a.RunAsync(func() func() {
		if err := client.UseSpace(space); err != nil {
			return func() {
				a.setStatus("USE space failed: "+err.Error(), true)
			}
		}
		schema, err := client.LoadSchema(space)
		return func() {
			a.CurrentSpace = space
			if err == nil {
				a.Schema = schema
			}
			// Clear stale graph/results from previous space so mismatched data never lingers
			a.clearTabResult(a.CurrentTab())
			tagCnt, edgeCnt := 0, 0
			if a.Schema != nil {
				tagCnt = len(a.Schema.Tags)
				edgeCnt = len(a.Schema.Edges)
			}
			a.setStatus(fmt.Sprintf("Active space: %s (%d tags, %d edges)", space, tagCnt, edgeCnt), false)
		}
	})
}

// RefreshSchema reloads spaces and schema metadata for the active space.
func (a *App) RefreshSchema() {
	if a.Client == nil {
		return
	}
	client := a.Client
	space := a.CurrentSpace
	a.setStatus("Refreshing schema…", false)
	a.RunAsync(func() func() {
		spaces, _ := client.Spaces()
		var schema *model.SpaceSchema
		if space != "" {
			schema, _ = client.LoadSchema(space)
		}
		return func() {
			if len(spaces) > 0 {
				a.Spaces = spaces
			}
			if schema != nil {
				a.Schema = schema
			}
			a.setStatus("Schema refreshed", false)
		}
	})
}

// ActiveQueryText returns the selected text in the editor if non-empty, else the whole buffer.
func (a *App) ActiveQueryText(tab *QueryTab) string {
	if tab == nil || tab.Editor == nil || tab.Editor.Buffer == nil {
		return ""
	}
	sels := tab.Editor.Selections()
	if len(sels) > 0 {
		s := sels[0]
		lo, hi := s.Anchor, s.Head
		if lo > hi {
			lo, hi = hi, lo
		}
		if hi > lo {
			if sub := strings.TrimSpace(tab.Editor.Buffer.Slice(lo, hi)); sub != "" {
				return sub
			}
		}
	}
	return strings.TrimSpace(tab.Editor.Buffer.String())
}

// RunCurrentQuery executes the active tab's query asynchronously.
func (a *App) RunCurrentQuery() {
	tab := a.CurrentTab()
	query := a.ActiveQueryText(tab)
	a.RunQueryInTab(tab, query)
}

// RunQueryInTab runs an explicit nGQL statement in the target tab.
func (a *App) RunQueryInTab(tab *QueryTab, query string) {
	if tab == nil {
		return
	}
	query = strings.TrimSpace(query)
	if query == "" {
		a.setStatus("Enter an nGQL query to execute", true)
		return
	}
	if a.Client == nil || !a.Client.Connected() {
		a.ConnModalOpen = true
		a.setStatus("Please connect to a NebulaGraph cluster or Demo dataset first", true)
		return
	}
	if tab.Running {
		return
	}

	tab.Running = true
	client := a.Client
	a.setStatus("Executing nGQL…", false)

	a.RunAsync(func() func() {
		qr, err := client.Execute(query)
		var newSchema *model.SpaceSchema
		var newSpace string
		if err == nil && qr != nil && qr.Error == "" {
			newSpace = client.CurrentSpace()
			// If query switched spaces or altered schema, refresh schema metadata automatically
			up := strings.ToUpper(query)
			if newSpace != "" && (strings.Contains(up, "USE ") || strings.Contains(up, "CREATE ") || strings.Contains(up, "DROP ") || strings.Contains(up, "ALTER ")) {
				newSchema, _ = client.LoadSchema(newSpace)
			}
		}
		return func() {
			tab.Running = false
			if err != nil {
				qr = &model.QueryResult{
					Query:      query,
					Space:      a.CurrentSpace,
					Error:      err.Error(),
					ExecutedAt: time.Now(),
				}
			}
			a.applyQueryResult(tab, qr)
			if newSpace != "" && newSpace != a.CurrentSpace {
				a.CurrentSpace = newSpace
			}
			if newSchema != nil {
				a.Schema = newSchema
			}
		}
	})
}

func (a *App) executeQuerySync(tab *QueryTab, query string) {
	if a.Client == nil || tab == nil {
		return
	}
	qr, err := a.Client.Execute(query)
	if err != nil {
		qr = &model.QueryResult{Query: query, Space: a.CurrentSpace, Error: err.Error(), ExecutedAt: time.Now()}
	}
	a.applyQueryResult(tab, qr)
}

func (a *App) applyQueryResult(tab *QueryTab, qr *model.QueryResult) {
	tab.Result = qr
	tab.SelectedRow = -1
	tab.SelectedVertex = nil
	tab.SelectedEdge = nil

	// Record history
	hist := model.QueryHistoryItem{
		ID:         fmt.Sprintf("h-%d", time.Now().UnixNano()),
		Query:      qr.Query,
		Space:      qr.Space,
		Connection: a.ActiveConn.Name,
		LatencyMs:  qr.LatencyMs,
		RowCount:   len(qr.Rows),
		Error:      qr.Error,
		Timestamp:  time.Now(),
	}
	a.Config.History = append([]model.QueryHistoryItem{hist}, a.Config.History...)
	if len(a.Config.History) > 200 {
		a.Config.History = a.Config.History[:200]
	}
	_ = a.Store.Save(a.Config)

	if qr.Error != "" {
		// Always clear stale graph/JSON from previous queries so old data never masquerades as current output
		tab.Graph.SetGraph(model.NewGraphData())
		if tab.JSONEditor != nil && tab.JSONEditor.Buffer != nil {
			tab.JSONEditor.Buffer.Reset("{}")
		}
		tab.Editor.SetDiagnostics(tab.Editor.Buffer.Version(), []editor.Diagnostic{{
			Start:    0,
			End:      len(tab.Editor.Buffer.String()),
			Severity: editor.SeverityError,
			Message:  qr.Error,
			Source:   "NebulaGraph",
		}})
		a.setStatus("Query error: "+qr.Error, true)
		return
	}
	tab.Editor.SetDiagnostics(tab.Editor.Buffer.Version(), nil)

	if tab.JSONEditor != nil && tab.JSONEditor.Buffer != nil {
		tab.JSONEditor.Buffer.Reset(qr.RawJSON)
	}

	// Default VID column selector for scalar-to-graph exploration
	if len(qr.Columns) > 0 {
		found := false
		for _, c := range qr.Columns {
			if c == tab.VIDColChoice {
				found = true
				break
			}
		}
		if !found {
			tab.VIDColChoice = qr.Columns[0]
		}
	} else {
		tab.VIDColChoice = ""
	}

	// ALWAYS synchronize tab.Graph with the current query result (clears graph if 0 vertices/edges!)
	tab.Graph.SetGraph(qr.Graph)

	if qr.HasGraph() {
		if qr.HasPlan() {
			tab.ResultTab = 3
		} else {
			tab.ResultTab = 0 // Auto-focus Graph View when vertices/edges are returned!
		}
	} else if qr.HasPlan() {
		tab.ResultTab = 3
	} else {
		tab.ResultTab = 1 // Table view for scalar / SHOW / DESCRIBE queries
	}

	vCount, eCount := 0, 0
	if qr.Graph != nil {
		vCount = len(qr.Graph.Vertices)
		eCount = len(qr.Graph.Edges)
	}
	a.setStatus(fmt.Sprintf("OK • %d rows • %d vertices, %d edges • %.2f ms",
		len(qr.Rows), vCount, eCount, qr.LatencyMs), false)
}

// ExpandVertex executes a strictly LIMIT-bounded 1..N hop expansion query and merges newly
// discovered nodes/edges smoothly into the active graph canvas without disturbing existing positions.
func (a *App) ExpandVertex(tab *QueryTab, v *model.Vertex, direction string, steps int) {
	if tab == nil || v == nil || a.Client == nil || tab.Expanding {
		return
	}
	client := a.Client
	limit := a.ExpandLimit
	if limit <= 0 {
		limit = 50
	}
	tab.Expanding = true
	a.setStatus(fmt.Sprintf("Expanding %s (%s, %d-hop, limit %d)…", v.ID, direction, steps, limit), false)

	a.RunAsync(func() func() {
		qr, err := client.ExpandVertex(v.ID, v.IsIntID, direction, steps, limit)
		return func() {
			tab.Expanding = false
			if err != nil {
				a.setStatus("Expand failed: "+err.Error(), true)
				return
			}
			if qr.Error != "" {
				a.setStatus("Expand error: "+qr.Error, true)
				return
			}
			addedV, addedE := tab.Graph.MergeGraph(qr.Graph, v.ID)
			if addedV == 0 && addedE == 0 {
				a.setStatus(fmt.Sprintf("No additional %s neighbors found for %s (limit %d)", direction, v.ID, limit), false)
				a.ToastQueue = "No new neighbors found"
			} else {
				hint := ""
				if len(qr.Rows) >= limit {
					hint = fmt.Sprintf(" [capped at limit %d]", limit)
				}
				a.setStatus(fmt.Sprintf("Expanded %s: +%d vertices, +%d edges%s (%.2f ms)",
					v.ID, addedV, addedE, hint, qr.LatencyMs), false)
				a.ToastQueue = fmt.Sprintf("+%d vertices, +%d edges", addedV, addedE)
			}
		}
	})
}

// LoadQueryIntoEditor replaces the current tab's editor text and optionally runs it immediately.
func (a *App) LoadQueryIntoEditor(query string, autoRun bool) {
	tab := a.CurrentTab()
	tab.Editor.Buffer.Set(query)
	if autoRun {
		a.RunQueryInTab(tab, query)
	}
}

// SaveCurrentAsSnippet stores the active query in the user's favorites.
func (a *App) SaveCurrentAsSnippet(title, category string) {
	q := a.ActiveQueryText(a.CurrentTab())
	if q == "" || strings.TrimSpace(title) == "" {
		return
	}
	if strings.TrimSpace(category) == "" {
		category = "Custom"
	}
	a.Config.Snippets = append([]model.SavedSnippet{{
		ID:       fmt.Sprintf("snip-%d", time.Now().UnixNano()),
		Title:    strings.TrimSpace(title),
		Category: strings.TrimSpace(category),
		Query:    q,
	}}, a.Config.Snippets...)
	_ = a.Store.Save(a.Config)
	a.SaveSnippetOpen = false
	a.SnippetTitle = ""
	a.ToastQueue = "Snippet saved"
}

// OpenExportPreview prepares the Export modal with formatted CSV, JSON, or SVG text.
func (a *App) OpenExportPreview(title, ext, lang, content string) {
	buf := editor.NewBuffer(content)
	if lang != "" {
		if l := editor.LanguageNamed(lang); l != nil {
			buf.SetLanguage(l)
		}
	}
	a.ExportTitle = title
	a.ExportExt = ext
	a.ExportContent = content
	a.ExportEditor = &editor.State{
		Buffer:   buf,
		ReadOnly: true,
		SoftWrap: false,
	}
	a.ExportModalOpen = true
}

// SaveExportToFile writes the exported content using the native OS save dialog.
func (a *App) SaveExportToFile() {
	if a.Win == nil {
		return
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.Win,
		Title:       a.ExportTitle,
		DefaultPath: "nebula-export." + a.ExportExt,
	})
	if err != nil || path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(a.ExportContent), 0o644); err != nil {
		a.setStatus("Save failed: "+err.Error(), true)
		return
	}
	a.ExportModalOpen = false
	a.ToastQueue = "Saved to " + path
	a.setStatus("Exported to "+path, false)
}

// FormatResultCSV serializes the query table into RFC-4180 CSV.
func FormatResultCSV(qr *model.QueryResult) string {
	if qr == nil {
		return ""
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(qr.Columns)
	for _, row := range qr.Rows {
		_ = w.Write(row)
	}
	w.Flush()
	return b.String()
}

// SortedFilteredRows returns row indices matching TableFilter sorted by TableSort,
// cached across frames while inputs are unchanged.
func (t *QueryTab) SortedFilteredRows() []int {
	if t == nil || t.Result == nil {
		return nil
	}
	if t.cachedRowResult == t.Result && t.cachedRowFilter == t.TableFilter && t.cachedRowSort == t.TableSort && t.cachedRowIdx != nil {
		return t.cachedRowIdx
	}
	q := strings.ToLower(strings.TrimSpace(t.TableFilter))
	indices := make([]int, 0, len(t.Result.Rows))
	for i, row := range t.Result.Rows {
		if q == "" {
			indices = append(indices, i)
			continue
		}
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), q) {
				indices = append(indices, i)
				break
			}
		}
	}
	if t.TableSort.Column != "" {
		colIdx := -1
		for i, col := range t.Result.Columns {
			if col == t.TableSort.Column {
				colIdx = i
				break
			}
		}
		if colIdx >= 0 {
			desc := t.TableSort.Descending
			sort.SliceStable(indices, func(i, j int) bool {
				a, b := "", ""
				if colIdx < len(t.Result.Rows[indices[i]]) {
					a = t.Result.Rows[indices[i]][colIdx]
				}
				if colIdx < len(t.Result.Rows[indices[j]]) {
					b = t.Result.Rows[indices[j]][colIdx]
				}
				// Numeric comparison when both cells parse as numbers
				fa, ea := strconv.ParseFloat(a, 64)
				fb, eb := strconv.ParseFloat(b, 64)
				var less bool
				if ea == nil && eb == nil {
					less = fa < fb
				} else {
					less = a < b
				}
				if desc {
					return !less
				}
				return less
			})
		}
	}
	t.cachedRowResult = t.Result
	t.cachedRowFilter = t.TableFilter
	t.cachedRowSort = t.TableSort
	t.cachedRowIdx = indices
	return indices
}

func (a *App) setStatus(msg string, isErr bool) {
	a.StatusMsg = msg
	a.StatusIsErr = isErr
}

// clearTabResult wipes previous query result, graph canvas, and inspector selection for one tab.
func (a *App) clearTabResult(tab *QueryTab) {
	if tab == nil {
		return
	}
	tab.Result = nil
	tab.SelectedRow = -1
	tab.SelectedVertex = nil
	tab.SelectedEdge = nil
	tab.VIDColChoice = ""
	if tab.Graph != nil {
		tab.Graph.SetGraph(model.NewGraphData())
	}
	if tab.JSONEditor != nil && tab.JSONEditor.Buffer != nil {
		tab.JSONEditor.Buffer.Reset("{}")
	}
}

// clearAllTabResults wipes stale results across all tabs when switching connections.
// If connecting to a live (non-Demo) cluster, any leftover Demo starter query is also cleared.
func (a *App) clearAllTabResults(isDemo bool) {
	for _, tab := range a.Tabs {
		a.clearTabResult(tab)
		if !isDemo && tab.Editor != nil && tab.Editor.Buffer != nil {
			if strings.TrimSpace(tab.Editor.Buffer.String()) == strings.TrimSpace(demoStarterQuery) {
				tab.Editor.Buffer.Reset(a.SampleGraphQuery())
			}
		}
	}
}

// IsIntVIDSpace reports whether the active space uses INT64 vertex IDs.
func (a *App) IsIntVIDSpace() bool {
	if a.Schema == nil {
		return false
	}
	return strings.Contains(strings.ToUpper(a.Schema.VIDType), "INT")
}

// SampleGraphQuery builds a valid sample graph query using the active space's real schema.
func (a *App) SampleGraphQuery() string {
	if a.Schema != nil {
		if len(a.Schema.Edges) > 0 {
			return fmt.Sprintf("MATCH p=()-[e:`%s`]->()\nRETURN p\nLIMIT 25;", a.Schema.Edges[0].Name)
		}
		if len(a.Schema.Tags) > 0 {
			return fmt.Sprintf("MATCH (v:`%s`)\nRETURN v\nLIMIT 25;", a.Schema.Tags[0].Name)
		}
	}
	return "SHOW TAGS;"
}

// VisualizeColumnVIDs extracts up to 25 distinct VIDs from a scalar result column and executes
// a real GET SUBGRAPH (steps > 0) or FETCH PROP (steps == 0) query against the active cluster.
func (a *App) VisualizeColumnVIDs(tab *QueryTab, colName string, steps int) {
	if tab == nil || tab.Result == nil || len(tab.Result.Rows) == 0 {
		return
	}
	colIdx := -1
	for i, c := range tab.Result.Columns {
		if c == colName {
			colIdx = i
			break
		}
	}
	if colIdx < 0 {
		return
	}
	isInt := a.IsIntVIDSpace()
	seen := make(map[string]bool)
	var vids []string
	for _, row := range tab.Result.Rows {
		if colIdx >= len(row) {
			continue
		}
		val := strings.TrimSpace(row[colIdx])
		if val == "" || val == "NULL" || seen[val] {
			continue
		}
		seen[val] = true
		vids = append(vids, model.FormatNGQLVID(val, isInt))
		if len(vids) >= 25 {
			break
		}
	}
	if len(vids) == 0 {
		a.setStatus("No valid VIDs found in column "+colName, true)
		return
	}
	var stmt string
	if steps <= 0 {
		stmt = fmt.Sprintf("FETCH PROP ON * %s\nYIELD vertex AS v;", strings.Join(vids, ", "))
	} else {
		limit := a.ExpandLimit * 2
		if limit < 80 {
			limit = 80
		}
		hop := ""
		if steps > 1 {
			hop = fmt.Sprintf("*1..%d", steps)
		}
		stmt = fmt.Sprintf("MATCH p=(v)-[e%s]-(n)\nWHERE id(v) IN [%s]\nRETURN p\nLIMIT %d;", hop, strings.Join(vids, ", "), limit)
	}
	a.LoadQueryIntoEditor(stmt, true)
}

// ExploreScalarVID runs a strictly LIMIT-bounded 1-hop MATCH query for a single scalar VID.
func (a *App) ExploreScalarVID(tab *QueryTab, rawVal string) {
	rawVal = strings.TrimSpace(rawVal)
	if rawVal == "" || rawVal == "NULL" {
		return
	}
	limit := a.ExpandLimit
	if limit <= 0 {
		limit = 50
	}
	stmt := nebula.BuildExpandQuery(rawVal, a.IsIntVIDSpace(), "BOTH", 1, limit)
	a.LoadQueryIntoEditor(stmt, true)
}

// Tr returns the active localized string bundle.
func (a *App) Tr() *i18n.Strings {
	if a.Config == nil {
		return i18n.Get("en")
	}
	return i18n.Get(a.Config.Language)
}

// ToggleLanguage switches between English ("en") and Simplified Chinese ("zh") and persists the choice.
func (a *App) ToggleLanguage() {
	if a.Config.Language == string(i18n.ZH) {
		a.Config.Language = string(i18n.EN)
	} else {
		a.Config.Language = string(i18n.ZH)
	}
	_ = a.Store.Save(a.Config)
}

// CycleTheme cycles between Dark, Light, and System themes.
func (a *App) CycleTheme() {
	switch a.Config.ThemeMode {
	case "dark":
		a.Config.ThemeMode = "light"
	case "light":
		a.Config.ThemeMode = "system"
	default:
		a.Config.ThemeMode = "dark"
	}
	_ = a.Store.Save(a.Config)
}

func wordAtOffset(src string, offset int) (word string, start, end int) {
	if offset < 0 || offset > len(src) {
		return "", 0, 0
	}
	isWord := func(b byte) bool {
		return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
	}
	start = offset
	for start > 0 && isWord(src[start-1]) {
		start--
	}
	end = offset
	for end < len(src) && isWord(src[end]) {
		end++
	}
	return src[start:end], start, end
}
