package app

import (
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"nebula_client/internal/model"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	store := &model.ConfigStore{Path: filepath.Join(t.TempDir(), "config.json")}
	a := NewApp(store)
	// Make async actions synchronous and deterministic inside headless UI tests
	a.RunAsync = func(work func() func()) {
		if commit := work(); commit != nil {
			commit()
		}
	}
	return a
}

func saveSnapshot(t *testing.T, tt *ui.Tester, filename string) {
	t.Helper()
	dir := os.Getenv("NEBULADESK_SNAPSHOT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	img := tt.Image()
	if img == nil {
		t.Fatalf("tester Image() returned nil for %s", filename)
	}
	outPath := filepath.Join(dir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create snapshot %s: %v", outPath, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png %s: %v", outPath, err)
	}
}

func TestAppEndToEndUIAndSnapshots(t *testing.T) {
	a := newTestApp(t)
	// Stop physics animation after initial pre-warm so Tester settles deterministically
	a.CurrentTab().Graph.Simulating = false

	tt := ui.NewTesterWith(a.View, 1380, 860, ui.TesterOptions{Dark: true})

	// 1. Verify initial dark-mode workspace loaded with Basketball Demo & Graph View
	for _, want := range []string{"NebulaDesk", "basketballplayer", "player", "team", "follow", "serve"} {
		if !tt.HasText(want) {
			t.Errorf("expected initial UI to show %q", want)
		}
	}
	// Select a top hub vertex to showcase the Vertex Inspector on the right!
	tab := a.CurrentTab()
	if nv, ok := tab.Graph.Nodes["player100"]; ok {
		tab.Graph.SelectedNodeID = "player100"
		tab.SelectedVertex = nv.Vertex
		tt.Frame()
	}
	saveSnapshot(t, tt, "01_dark_graph_inspector.png")

	// 2. Test interactive Expand Both from Vertex Inspector
	if err := tt.Click("Expand Both"); err != nil {
		t.Fatalf("Click Expand Both: %v", err)
	}
	tab.Graph.Simulating = false
	tt.Frame()

	// 3. Switch to Table View and verify tabular results
	tab.ResultTab = 1
	tt.Frame()
	if !tt.HasText("Copy CSV") {
		t.Errorf("expected Table view toolbar with Copy CSV")
	}
	saveSnapshot(t, tt, "02_dark_table_view.png")

	// 4. Execute EXPLAIN query and inspect Plan View
	a.LoadQueryIntoEditor("EXPLAIN MATCH p=(v:player)-[e:follow]->(m:player) WHERE v.player.age > 35 RETURN p LIMIT 15;", true)
	tab.Graph.Simulating = false
	tt.Frame()
	if !tt.HasText("Physical Execution Plan") || !tt.HasText("IndexScan") {
		t.Errorf("expected Plan view with IndexScan operator")
	}
	saveSnapshot(t, tt, "03_dark_explain_plan.png")

	// 5. Switch to Light Theme + Circular Graph Layout
	a.LoadQueryIntoEditor("MATCH p=(v:player)-[e:follow|serve]->(n) RETURN p LIMIT 30;", true)
	tab.Graph.ApplyLayout(1) // Circular
	tab.Graph.NeedsFit = true
	a.Config.ThemeMode = "light"
	tt.SetDark(false)
	tt.Frame()
	saveSnapshot(t, tt, "04_light_circular_graph.png")

	// 6. Open Connection Manager modal & snapshot
	a.Config.ThemeMode = "dark"
	tt.SetDark(true)
	a.openConnModal()
	tt.Frame()
	if !tt.HasText("NebulaGraph Connections") {
		t.Errorf("expected Connection Manager modal open")
	}
	saveSnapshot(t, tt, "05_connection_modal.png")

	// 7. Close modal, switch sidebar to Snippets & History
	a.ConnModalOpen = false
	a.SidebarTab = 2
	tt.Frame()
	if !tt.HasText("Find Shortest Path") {
		t.Errorf("expected Snippets sidebar to list Find Shortest Path")
	}

	// 8. Switch Space to tech_ecosystem and verify schema updates & stale graph is cleared!
	a.SwitchSpace("tech_ecosystem")
	a.SidebarTab = 0
	tt.Frame()
	if !tt.HasText("tech_ecosystem") || !tt.HasText("built_with") {
		t.Errorf("expected tech_ecosystem schema to load after space switch")
	}
	if !tab.Graph.Graph.Empty() {
		t.Errorf("expected stale graph from previous space to be cleared on space switch, got %dV", len(tab.Graph.Graph.Vertices))
	}
}

func TestScalarQueryClearsStaleGraph(t *testing.T) {
	a := newTestApp(t)
	tab := a.CurrentTab()
	tab.Graph.Simulating = false
	if tab.Graph.Graph.Empty() {
		t.Fatal("expected starter demo query to populate graph")
	}

	tt := ui.NewTesterWith(a.View, 1380, 860, ui.TesterOptions{Dark: true})

	// Run a scalar-only query (SHOW STATS returns tabular rows with 0 vertices/edges)
	a.LoadQueryIntoEditor("SHOW STATS;", true)
	tab.ResultTab = 0 // User clicks back to Graph tab
	tt.Frame()

	if !tab.Graph.Graph.Empty() {
		t.Fatalf("stale graph leaked after scalar query: got %d vertices, %d edges",
			len(tab.Graph.Graph.Vertices), len(tab.Graph.Graph.Edges))
	}
	if tt.HasText("Tim Duncan") {
		t.Errorf("stale demo vertex 'Tim Duncan' still visible in UI after scalar query")
	}
	if !tt.HasText("Query 1-Hop Subgraph") {
		t.Errorf("expected scalar-to-graph helper button in empty graph overlay")
	}
	saveSnapshot(t, tt, "06_scalar_clears_graph.png")
}

func TestLanguageToggleAndSchemaIndexIcons(t *testing.T) {
	a := newTestApp(t)
	a.CurrentTab().Graph.Simulating = false
	// Expand player, team, and follow in schema sidebar to verify inline index icons
	a.TagExpanded["player"] = true
	a.TagExpanded["team"] = true
	a.EdgeExpanded["follow"] = true

	tt := ui.NewTesterWith(a.View, 1380, 860, ui.TesterOptions{Dark: true})

	// Click language switcher button ("EN" -> "中文")
	if err := tt.Click("EN"); err != nil {
		t.Fatalf("Click language button EN: %v", err)
	}
	if a.Config.Language != "zh" {
		t.Fatalf("expected language 'zh', got %q", a.Config.Language)
	}
	for _, want := range []string{"中文", "侧边栏", "属性面板", "图空间:", "点类型 TAGS", "力导向", "图谱概览"} {
		if !tt.HasText(want) {
			t.Errorf("expected Chinese UI label %q after switching language", want)
		}
	}
	saveSnapshot(t, tt, "07_zh_schema_index_icons.png")

	// Toggle back to English
	if err := tt.Click("中文"); err != nil {
		t.Fatalf("Click language button 中文: %v", err)
	}
	if a.Config.Language != "en" || !tt.HasText("Sidebar") || !tt.HasText("Graph Overview") {
		t.Errorf("expected English UI restored after toggling back")
	}
}

func TestNGQLContextAutocompleteUI(t *testing.T) {
	a := newTestApp(t)
	tab := a.CurrentTab()
	tab.Graph.Simulating = false

	tt := ui.NewTesterWith(a.View, 1380, 860, ui.TesterOptions{Dark: true})

	// 1. Vertex variable dot completion: `MATCH (v:player)-[e:follow]->(n) WHERE v.`
	query := "MATCH (v:player)-[e:follow]->(n) WHERE v."
	tab.Editor.Buffer.Set(query)
	tab.Editor.Select(len(query), len(query))
	tab.Editor.Run(editor.ShowCompletions)
	tt.Frame()

	for _, want := range []string{"player.name", "player.age", "Insert"} {
		if !tt.HasText(want) {
			t.Errorf("expected completion popup to render %q", want)
		}
	}
	saveSnapshot(t, tt, "08_ngql_autocomplete.png")

	// 2. Accept the selected completion (`player.name`) via Newline (Enter)
	tab.Editor.Run(editor.Newline)
	tt.Frame()
	got := tab.Editor.Buffer.String()
	if !strings.HasSuffix(got, "v.player.name") {
		t.Fatalf("expected accepting completion to produce suffix 'v.player.name', got %q", got)
	}
}

func TestAIReActGenerationAndModelConfigUI(t *testing.T) {
	var reactTurn int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/v1/models" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": "list",
				"data": []any{
					map[string]any{"id": "deepseek-chat"},
					map[string]any{"id": "deepseek-reasoner"},
				},
			})
			return
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Ping from Test Connection
		if body.MaxTokens == 16 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{"role": "assistant", "content": "OK"},
				}},
			})
			return
		}

		step := atomic.AddInt32(&reactTurn, 1)
		switch step {
		case 1:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Let me sample player vertices to inspect property values.",
						"tool_calls": []any{map[string]any{
							"id":   "tc1",
							"type": "function",
							"function": map[string]any{
								"name":      "sample_tag",
								"arguments": `{"tag_name":"player","limit":3}`,
							},
						}},
					},
				}},
			})
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Now verify the MATCH path query for players older than 35.",
						"tool_calls": []any{map[string]any{
							"id":   "tc2",
							"type": "function",
							"function": map[string]any{
								"name":      "run_readonly_ngql",
								"arguments": `{"ngql":"MATCH p=(v:player)-[e:follow]->(n:player) WHERE v.player.age > 35 RETURN p LIMIT 12;"}`,
							},
						}},
					},
				}},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []any{map[string]any{
							"id":   "tc3",
							"type": "function",
							"function": map[string]any{
								"name":      "finish",
								"arguments": `{"ngql":"MATCH p=(v:player)-[e:follow]->(n:player) WHERE v.player.age > 35 RETURN p LIMIT 12;","explanation":"Finds follow paths starting from players older than 35 and returns visualizable paths."}`,
							},
						}},
					},
				}},
			})
		}
	}))
	defer srv.Close()

	a := newTestApp(t)
	a.CurrentTab().Graph.Simulating = false
	tt := ui.NewTesterWith(a.View, 1380, 860, ui.TesterOptions{Dark: true})

	// 1. Click "✦ AI" in top toolbar -> should open AI panel showing unconfigured setup card
	if err := tt.Click("✦ AI"); err != nil {
		t.Fatalf("Click AI toolbar button: %v", err)
	}
	if !tt.HasText("Connect an AI Model") {
		t.Fatalf("expected unconfigured AI onboarding card")
	}

	// 2. Open AI Model Settings modal, fetch available models via GET /models, test connection, and save
	a.OpenAIModal()
	a.AIDraft.Provider = "DeepSeek"
	a.AIDraft.BaseURL = srv.URL + "/v1"
	a.AIDraft.APIKey = "demo-secret"
	a.AIDraft.Model = ""
	tt.Frame()
	if err := tt.Click("↻ Fetch Models"); err != nil {
		t.Fatalf("Click Fetch Models: %v", err)
	}
	if len(a.AIFetchedModels) != 2 || a.AIDraft.Model != "deepseek-chat" {
		t.Fatalf("expected 2 fetched models and auto-selected deepseek-chat, got models=%v selected=%q", a.AIFetchedModels, a.AIDraft.Model)
	}
	a.TestAIConnection()
	tt.Frame()
	if a.AITestIsErr || !strings.Contains(a.AITestMsg, "Connected") {
		t.Fatalf("expected successful AI connection test, got isErr=%v msg=%q", a.AITestIsErr, a.AITestMsg)
	}
	saveSnapshot(t, tt, "10_ai_model_modal.png")

	a.SaveAIDraft()
	tt.Frame()
	if !a.Config.AI.Configured() || a.Config.AI.Model != "deepseek-chat" {
		t.Fatalf("expected saved AI model config, got %+v", a.Config.AI)
	}

	// 3. Run ReAct nGQL generation
	a.AIPrompt = "Find follow paths for players older than 35"
	a.StartAIGenerate()
	tt.Frame()

	for _, want := range []string{"Exploration Steps", "Sample Vertices", "Verify Query", "Generated nGQL", "Verified", "Load & Run"} {
		if !tt.HasText(want) {
			t.Errorf("expected AI panel to render %q", want)
		}
	}
	saveSnapshot(t, tt, "09_ai_react_panel.png")

	// 4. Click "Load & Run" to execute the generated nGQL into the active tab!
	if err := tt.Click("Load & Run"); err != nil {
		t.Fatalf("Click Load & Run: %v", err)
	}
	a.CurrentTab().Graph.Simulating = false
	tt.Frame()
	if !strings.Contains(a.CurrentTab().Editor.Buffer.String(), "v.player.age > 35") {
		t.Fatalf("expected generated nGQL loaded in editor, got %q", a.CurrentTab().Editor.Buffer.String())
	}
	if a.CurrentTab().Graph.Graph.Empty() {
		t.Fatalf("expected Load & Run to execute query and populate graph canvas")
	}
}

func TestTableCellMagnifierModal(t *testing.T) {
	a := newTestApp(t)
	tab := a.CurrentTab()
	tab.Graph.Simulating = false
	tab.ResultTab = 1 // Table view
	tab.SelectedRow = 0

	tt := ui.NewTesterWith(a.View, 1380, 860, ui.TesterOptions{Dark: true})
	if !tt.HasText("Inspect Row") {
		t.Fatalf("expected 'Inspect Row' magnifier button in table toolbar when row 0 is selected")
	}
	saveSnapshot(t, tt, "11_table_with_magnifiers.png")

	// Open cell magnifier on the last column ("path") of row 0
	pathCol := len(tab.Result.Columns) - 1
	a.OpenCellPreview(tab, 0, pathCol)
	tt.Frame()
	if !a.CellPreviewOpen || !tt.HasText("Row #1") || !tt.HasText("Format Path / JSON") {
		t.Fatalf("expected cell magnifier modal to render Row #1 and format toggle")
	}
	saveSnapshot(t, tt, "12_cell_magnifier_modal.png")

	// Switch to Full Row tab & step to Next row
	if err := tt.Click("Next →"); err != nil {
		t.Fatalf("Click Next row: %v", err)
	}
	if a.CellPreviewRowPos != 1 {
		t.Fatalf("expected CellPreviewRowPos=1 after Next, got %d", a.CellPreviewRowPos)
	}
	a.CellPreviewTab = 1
	tt.Frame()
	if !tt.HasText("Row #2") {
		t.Fatalf("expected Full Row view for Row #2")
	}
}
