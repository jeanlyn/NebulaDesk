package nebula

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nebula_client/internal/model"
)

// DemoClient provides a zero-friction, rich local NebulaGraph simulator loaded with
// the classic NebulaGraph `basketballplayer` dataset (plus an extra `social_network` space).
// Users can immediately explore graph visualization, nGQL queries, node expansion,
// shortest path, schema inspection, and EXPLAIN plans before connecting to a live cluster.
type DemoClient struct {
	mu      sync.Mutex
	profile model.ConnectionProfile
	space   string
	spaces  map[string]*demoSpace
	closed  bool
}

type demoSpace struct {
	name   string
	schema *model.SpaceSchema
	graph  *model.GraphData
}

// NewDemoClient initializes the built-in interactive demo NebulaGraph engine.
func NewDemoClient(profile model.ConnectionProfile) *DemoClient {
	spaces := map[string]*demoSpace{
		"basketballplayer": buildBasketballSpace(),
		"tech_ecosystem":   buildTechEcosystemSpace(),
	}
	space := profile.DefaultSpace
	if _, ok := spaces[space]; !ok {
		space = "basketballplayer"
	}
	return &DemoClient{
		profile: profile,
		space:   space,
		spaces:  spaces,
	}
}

func (d *DemoClient) Profile() model.ConnectionProfile { return d.profile }

func (d *DemoClient) Connected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return !d.closed
}

func (d *DemoClient) CurrentSpace() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.space
}

func (d *DemoClient) Spaces() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, 0, len(d.spaces))
	for k := range d.spaces {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func (d *DemoClient) UseSpace(space string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.spaces[space]; !ok {
		return fmt.Errorf("SpaceNotFound: space `%s` does not exist in demo engine", space)
	}
	d.space = space
	return nil
}

func (d *DemoClient) LoadSchema(space string) (*model.SpaceSchema, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if space == "" {
		space = d.space
	}
	sp, ok := d.spaces[space]
	if !ok {
		return nil, fmt.Errorf("SpaceNotFound: `%s`", space)
	}
	d.space = space
	return sp.schema, nil
}

func (d *DemoClient) ExpandVertex(vid string, isInt bool, direction string, steps int, limit int) (*model.QueryResult, error) {
	stmt := BuildExpandQuery(vid, isInt, direction, steps, limit)
	return d.Execute(stmt)
}

func (d *DemoClient) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
}

func (d *DemoClient) Execute(rawStmt string) (*model.QueryResult, error) {
	start := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, fmt.Errorf("demo client closed")
	}

	stmt := stripComments(rawStmt)
	stmt = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(stmt), ";"))
	if stmt == "" {
		return &model.QueryResult{
			Query:      rawStmt,
			Space:      d.space,
			Error:      "Empty query statement",
			ExecutedAt: time.Now(),
		}, nil
	}

	// Support semicolon-chained statements like "USE tech_ecosystem; MATCH ..."
	parts := splitStatements(stmt)
	var qr *model.QueryResult
	for _, p := range parts {
		qr = d.execSingle(p)
		if qr.Error != "" {
			break
		}
	}
	if qr == nil {
		qr = &model.QueryResult{Query: rawStmt, Space: d.space, Graph: model.NewGraphData()}
	}
	qr.Query = rawStmt
	qr.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	if qr.LatencyMs < 0.25 {
		qr.LatencyMs = 0.85 // realistic sub-millisecond display
	}
	qr.ExecutedAt = time.Now()
	if qr.RawJSON == "" {
		qr.RawJSON = buildResultJSON(qr)
	}
	return qr, nil
}

func (d *DemoClient) execSingle(stmt string) *model.QueryResult {
	sp := d.spaces[d.space]
	upper := strings.ToUpper(strings.TrimSpace(stmt))

	// EXPLAIN / PROFILE wrapper
	if strings.HasPrefix(upper, "EXPLAIN ") || strings.HasPrefix(upper, "PROFILE ") {
		isProfile := strings.HasPrefix(upper, "PROFILE ")
		inner := strings.TrimSpace(stmt[8:])
		innerRes := d.execSingle(inner)
		innerRes.Plan = buildDemoPlan(inner, len(innerRes.Rows), isProfile)
		return innerRes
	}

	// USE <space>
	if strings.HasPrefix(upper, "USE ") {
		target := strings.Trim(strings.TrimSpace(stmt[4:]), "`\"'")
		if _, ok := d.spaces[target]; !ok {
			return errorResult(stmt, d.space, fmt.Sprintf("SpaceNotFound: `%s`", target))
		}
		d.space = target
		return &model.QueryResult{
			Query:   stmt,
			Space:   d.space,
			Columns: []string{"Space"},
			Rows:    [][]string{{d.space}},
			Graph:   model.NewGraphData(),
			Comment: "Switched space to " + d.space,
		}
	}

	switch {
	case upper == "SHOW SPACES":
		var rows [][]string
		names := make([]string, 0, len(d.spaces))
		for k := range d.spaces {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			rows = append(rows, []string{k})
		}
		return tableResult(stmt, d.space, []string{"Name"}, rows)

	case upper == "SHOW TAGS":
		var rows [][]string
		for _, t := range sp.schema.Tags {
			rows = append(rows, []string{t.Name})
		}
		return tableResult(stmt, d.space, []string{"Name"}, rows)

	case upper == "SHOW EDGES":
		var rows [][]string
		for _, e := range sp.schema.Edges {
			rows = append(rows, []string{e.Name})
		}
		return tableResult(stmt, d.space, []string{"Name"}, rows)

	case upper == "SHOW TAG INDEXES" || upper == "SHOW EDGE INDEXES":
		want := "TAG"
		if strings.Contains(upper, "EDGE") {
			want = "EDGE"
		}
		var rows [][]string
		for _, idx := range sp.schema.Indexes {
			if idx.Kind == want {
				rows = append(rows, []string{idx.Name, idx.Target, "[" + strings.Join(idx.Fields, ", ") + "]"})
			}
		}
		return tableResult(stmt, d.space, []string{"Index Name", "By " + want, "Columns"}, rows)

	case upper == "SHOW STATS":
		var rows [][]string
		for _, t := range sp.schema.Tags {
			rows = append(rows, []string{"Tag", t.Name, strconv.FormatInt(t.Count, 10)})
		}
		for _, e := range sp.schema.Edges {
			rows = append(rows, []string{"Edge", e.Name, strconv.FormatInt(e.Count, 10)})
		}
		rows = append(rows, []string{"Space", "vertices", strconv.FormatInt(sp.schema.TotalVertices, 10)})
		rows = append(rows, []string{"Space", "edges", strconv.FormatInt(sp.schema.TotalEdges, 10)})
		return tableResult(stmt, d.space, []string{"Type", "Name", "Count"}, rows)

	case strings.HasPrefix(upper, "DESCRIBE SPACE ") || strings.HasPrefix(upper, "DESC SPACE "):
		return tableResult(stmt, d.space,
			[]string{"ID", "Name", "Partition Number", "Replica Factor", "Charset", "Collate", "Vid Type"},
			[][]string{{"1", sp.name, strconv.Itoa(sp.schema.PartitionNum), strconv.Itoa(sp.schema.ReplicaFactor), "utf8", "utf8_bin", sp.schema.VIDType}},
		)

	case strings.HasPrefix(upper, "DESCRIBE TAG ") || strings.HasPrefix(upper, "DESC TAG "):
		name := lastIdent(stmt)
		for _, t := range sp.schema.Tags {
			if strings.EqualFold(t.Name, name) {
				var rows [][]string
				for _, p := range t.Properties {
					rows = append(rows, []string{p.Field, p.Type, p.Null, p.Default, p.Comment})
				}
				return tableResult(stmt, d.space, []string{"Field", "Type", "Null", "Default", "Comment"}, rows)
			}
		}
		return errorResult(stmt, d.space, fmt.Sprintf("TagNotFound: `%s`", name))

	case strings.HasPrefix(upper, "DESCRIBE EDGE ") || strings.HasPrefix(upper, "DESC EDGE "):
		name := lastIdent(stmt)
		for _, e := range sp.schema.Edges {
			if strings.EqualFold(e.Name, name) {
				var rows [][]string
				for _, p := range e.Properties {
					rows = append(rows, []string{p.Field, p.Type, p.Null, p.Default, p.Comment})
				}
				return tableResult(stmt, d.space, []string{"Field", "Type", "Null", "Default", "Comment"}, rows)
			}
		}
		return errorResult(stmt, d.space, fmt.Sprintf("EdgeNotFound: `%s`", name))

	case strings.HasPrefix(upper, "GET SUBGRAPH"):
		return d.execSubgraph(stmt, sp)

	case strings.HasPrefix(upper, "FIND SHORTEST PATH") || strings.HasPrefix(upper, "FIND ALL PATH"):
		return d.execFindPath(stmt, sp)

	case strings.HasPrefix(upper, "FETCH PROP"):
		return d.execFetch(stmt, sp)

	case strings.HasPrefix(upper, "GO "):
		return d.execGo(stmt, sp)

	case strings.HasPrefix(upper, "LOOKUP "):
		return d.execLookup(stmt, sp)

	case strings.HasPrefix(upper, "MATCH "):
		return d.execMatch(stmt, sp)
	}

	return errorResult(stmt, d.space,
		"Demo engine supports: MATCH, GO, GET SUBGRAPH, FIND SHORTEST PATH, FETCH PROP, LOOKUP, SHOW SPACES/TAGS/EDGES/STATS, DESCRIBE, USE, EXPLAIN, PROFILE. Connect to a live NebulaGraph cluster for full nGQL.")
}

var (
	reQuoted  = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)
	reLimit   = regexp.MustCompile(`(?i)\bLIMIT\s+(\d+)`)
	reSteps   = regexp.MustCompile(`(?i)\b(\d+)\s+STEPS?\b`)
	reGoRange = regexp.MustCompile(`(?i)\b(\d+)\s+TO\s+(\d+)\s+STEPS?\b`)
	reTagPat  = regexp.MustCompile(`\(\s*[a-zA-Z0-9_]*\s*:\s*` + "`?" + `([a-zA-Z0-9_]+)` + "`?")
	reEdgePat = regexp.MustCompile(`\[\s*[a-zA-Z0-9_]*\s*:\s*` + "`?" + `([a-zA-Z0-9_|` + "`" + `]+)` + "`?")
	reAgeCond = regexp.MustCompile(`(?i)age\s*(>=|<=|>|<|==|=)\s*(\d+)`)
	reIDCond  = regexp.MustCompile(`(?i)id\([^)]*\)\s*(?:==|=)\s*["']([^"']+)["']`)
	reIDIn    = regexp.MustCompile(`(?i)id\([^)]*\)\s+IN\s+\[([^\]]+)\]`)
	reNameEq  = regexp.MustCompile(`(?i)name\s*(?:==|=)\s*["']([^"']+)["']`)
)

func extractLimit(stmt string, def int) int {
	if m := reLimit.FindStringSubmatch(stmt); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func extractQuotedStrings(s string) []string {
	matches := reQuoted.FindAllStringSubmatch(s, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if m[1] != "" {
			out = append(out, m[1])
		} else if m[2] != "" {
			out = append(out, m[2])
		}
	}
	return out
}

func (d *DemoClient) execMatch(stmt string, sp *demoSpace) *model.QueryResult {
	limit := extractLimit(stmt, 35)
	hasEdge := strings.Contains(stmt, "-[") || strings.Contains(stmt, "->") || strings.Contains(stmt, "<-")
	dirIn := strings.Contains(stmt, "<-[") && !strings.Contains(stmt, "]->")
	dirOut := strings.Contains(stmt, "]->") && !strings.Contains(stmt, "<-[")

	// Extract optional edge type filters (e.g. follow|serve)
	edgeFilter := map[string]bool{}
	if m := reEdgePat.FindStringSubmatch(stmt); len(m) == 2 {
		for _, part := range strings.Split(m[1], "|") {
			part = strings.Trim(strings.TrimSpace(part), "`")
			if part != "" {
				edgeFilter[strings.ToLower(part)] = true
			}
		}
	}

	// Extract optional tag filter on first node
	var tagFilter string
	if m := reTagPat.FindStringSubmatch(stmt); len(m) == 2 {
		tagFilter = strings.ToLower(m[1])
	}

	// Optional WHERE conditions
	idFilterSet := map[string]bool{}
	if m := reIDCond.FindStringSubmatch(stmt); len(m) == 2 {
		idFilterSet[m[1]] = true
	}
	if m := reIDIn.FindStringSubmatch(stmt); len(m) == 2 {
		for _, qid := range extractQuotedStrings(m[1]) {
			idFilterSet[qid] = true
		}
	}
	var nameFilter string
	if m := reNameEq.FindStringSubmatch(stmt); len(m) == 2 {
		nameFilter = strings.ToLower(m[1])
	}
	ageOp, ageVal := "", 0
	if m := reAgeCond.FindStringSubmatch(stmt); len(m) == 3 {
		ageOp = m[1]
		ageVal, _ = strconv.Atoi(m[2])
	}

	matchVertex := func(v *model.Vertex) bool {
		if v == nil {
			return false
		}
		if len(idFilterSet) > 0 && !idFilterSet[v.ID] {
			return false
		}
		if tagFilter != "" {
			found := false
			for _, t := range v.Tags {
				if strings.EqualFold(t, tagFilter) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		if nameFilter != "" {
			n, _ := v.FindProperty("name")
			if !strings.Contains(strings.ToLower(n), nameFilter) {
				return false
			}
		}
		if ageOp != "" {
			raw, ok := v.FindProperty("age")
			if !ok {
				return false
			}
			a, err := strconv.Atoi(raw)
			if err != nil {
				return false
			}
			switch ageOp {
			case ">":
				if !(a > ageVal) {
					return false
				}
			case ">=":
				if !(a >= ageVal) {
					return false
				}
			case "<":
				if !(a < ageVal) {
					return false
				}
			case "<=":
				if !(a <= ageVal) {
					return false
				}
			case "==", "=":
				if !(a == ageVal) {
					return false
				}
			}
		}
		return true
	}

	g := model.NewGraphData()
	var rows [][]string

	if !hasEdge {
		// Vertex-only MATCH
		for _, id := range sp.graph.VertexOrder {
			v := sp.graph.Vertices[id]
			if !matchVertex(v) {
				continue
			}
			g.AddVertex(v)
			name, _ := v.FindProperty("name")
			age, _ := v.FindProperty("age")
			rows = append(rows, []string{v.ID, strings.Join(v.Tags, ", "), name, age})
			if len(rows) >= limit {
				break
			}
		}
		return &model.QueryResult{
			Query:   stmt,
			Space:   d.space,
			Columns: []string{"vid", "tags", "name", "age"},
			Rows:    rows,
			Graph:   g,
		}
	}

	// Edge / Path MATCH
	for _, eid := range sp.graph.EdgeOrder {
		e := sp.graph.Edges[eid]
		if len(edgeFilter) > 0 && !edgeFilter[strings.ToLower(e.Name)] {
			continue
		}
		src := sp.graph.Vertices[e.SrcID]
		dst := sp.graph.Vertices[e.DstID]

		matched := false
		switch {
		case dirOut:
			matched = matchVertex(src)
		case dirIn:
			matched = matchVertex(dst)
		default:
			matched = matchVertex(src) || matchVertex(dst)
		}
		if !matched {
			continue
		}

		g.AddVertex(src)
		g.AddVertex(dst)
		g.AddEdge(e)

		pathStr := fmt.Sprintf("(%s :%s)-[:%s@%d]->(%s :%s)",
			src.DisplayLabel("name"), src.PrimaryTag(),
			e.Name, e.Rank,
			dst.DisplayLabel("name"), dst.PrimaryTag())
		rows = append(rows, []string{
			src.ID,
			e.Name,
			dst.ID,
			formatPropMap(e.Properties),
			pathStr,
		})
		if len(rows) >= limit {
			break
		}
	}

	return &model.QueryResult{
		Query:   stmt,
		Space:   d.space,
		Columns: []string{"src", "edge", "dst", "properties", "path"},
		Rows:    rows,
		Graph:   g,
	}
}

func (d *DemoClient) execSubgraph(stmt string, sp *demoSpace) *model.QueryResult {
	steps := 1
	if m := reSteps.FindStringSubmatch(stmt); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			steps = n
		}
	}
	if steps > 4 {
		steps = 4
	}
	upper := strings.ToUpper(stmt)
	dir := "BOTH"
	if strings.Contains(upper, " OUT ") {
		dir = "OUT"
	} else if strings.Contains(upper, " IN ") {
		dir = "IN"
	}

	seeds := extractQuotedStrings(stmt)
	if len(seeds) == 0 && len(sp.graph.VertexOrder) > 0 {
		seeds = []string{sp.graph.VertexOrder[0]}
	}

	// Optional edge type restriction (e.g. BOTH follow, serve)
	edgeFilter := map[string]bool{}
	for _, es := range sp.schema.Edges {
		if strings.Contains(strings.ToLower(stmt), strings.ToLower(es.Name)) {
			edgeFilter[strings.ToLower(es.Name)] = true
		}
	}

	g := model.NewGraphData()
	frontier := map[string]bool{}
	visited := map[string]bool{}
	for _, s := range seeds {
		if v, ok := sp.graph.Vertices[s]; ok {
			g.AddVertex(v)
			frontier[s] = true
			visited[s] = true
		}
	}

	for step := 0; step < steps && len(frontier) > 0; step++ {
		nextFrontier := map[string]bool{}
		for _, eid := range sp.graph.EdgeOrder {
			e := sp.graph.Edges[eid]
			if len(edgeFilter) > 0 && !edgeFilter[strings.ToLower(e.Name)] {
				continue
			}
			take := false
			var neighbor string
			if (dir == "BOTH" || dir == "OUT") && frontier[e.SrcID] {
				take = true
				neighbor = e.DstID
			}
			if (dir == "BOTH" || dir == "IN") && frontier[e.DstID] {
				take = true
				neighbor = e.SrcID
			}
			if !take {
				continue
			}
			g.AddVertex(sp.graph.Vertices[e.SrcID])
			g.AddVertex(sp.graph.Vertices[e.DstID])
			g.AddEdge(e)
			if !visited[neighbor] {
				visited[neighbor] = true
				nextFrontier[neighbor] = true
			}
		}
		frontier = nextFrontier
	}

	var rows [][]string
	for _, eid := range g.EdgeOrder {
		e := g.Edges[eid]
		rows = append(rows, []string{e.SrcID, e.Name, e.DstID, formatPropMap(e.Properties)})
	}
	if len(rows) == 0 {
		for _, vid := range g.VertexOrder {
			v := g.Vertices[vid]
			rows = append(rows, []string{v.ID, strings.Join(v.Tags, ","), v.DisplayLabel("name"), ""})
		}
	}
	return &model.QueryResult{
		Query:   stmt,
		Space:   d.space,
		Columns: []string{"src", "edge", "dst", "properties"},
		Rows:    rows,
		Graph:   g,
	}
}

func (d *DemoClient) execGo(stmt string, sp *demoSpace) *model.QueryResult {
	minStep, maxStep := 1, 1
	if m := reGoRange.FindStringSubmatch(stmt); len(m) == 3 {
		minStep, _ = strconv.Atoi(m[1])
		maxStep, _ = strconv.Atoi(m[2])
	} else if m := reSteps.FindStringSubmatch(stmt); len(m) == 2 {
		maxStep, _ = strconv.Atoi(m[1])
		minStep = maxStep
	}
	if maxStep < 1 {
		maxStep = 1
	}
	if maxStep > 4 {
		maxStep = 4
	}
	upper := strings.ToUpper(stmt)
	bidirect := strings.Contains(upper, "BIDIRECT")
	reverse := strings.Contains(upper, "REVERSELY")

	seeds := extractQuotedStrings(stmt)
	if len(seeds) == 0 && len(sp.graph.VertexOrder) > 0 {
		seeds = []string{sp.graph.VertexOrder[0]}
	}

	edgeFilter := map[string]bool{}
	for _, es := range sp.schema.Edges {
		if strings.Contains(strings.ToLower(stmt), strings.ToLower(es.Name)) {
			edgeFilter[strings.ToLower(es.Name)] = true
		}
	}

	g := model.NewGraphData()
	frontier := map[string]bool{}
	for _, s := range seeds {
		if v, ok := sp.graph.Vertices[s]; ok {
			g.AddVertex(v)
			frontier[s] = true
		}
	}

	var rows [][]string
	for step := 1; step <= maxStep; step++ {
		next := map[string]bool{}
		for _, eid := range sp.graph.EdgeOrder {
			e := sp.graph.Edges[eid]
			if len(edgeFilter) > 0 && !edgeFilter[strings.ToLower(e.Name)] {
				continue
			}
			var from, to string
			if !reverse && frontier[e.SrcID] {
				from, to = e.SrcID, e.DstID
			} else if (reverse || bidirect) && frontier[e.DstID] {
				from, to = e.DstID, e.SrcID
			} else {
				continue
			}
			next[to] = true
			g.AddVertex(sp.graph.Vertices[e.SrcID])
			g.AddVertex(sp.graph.Vertices[e.DstID])
			g.AddEdge(e)
			if step >= minStep {
				targetName := to
				if tv := sp.graph.Vertices[to]; tv != nil {
					targetName = tv.DisplayLabel("name")
				}
				rows = append(rows, []string{strconv.Itoa(step), from, e.Name, to, targetName, formatPropMap(e.Properties)})
			}
		}
		frontier = next
	}

	return &model.QueryResult{
		Query:   stmt,
		Space:   d.space,
		Columns: []string{"step", "from_id", "edge", "to_id", "target_name", "edge_props"},
		Rows:    rows,
		Graph:   g,
	}
}

func (d *DemoClient) execFetch(stmt string, sp *demoSpace) *model.QueryResult {
	ids := extractQuotedStrings(stmt)
	g := model.NewGraphData()
	var rows [][]string
	for _, id := range ids {
		if v, ok := sp.graph.Vertices[id]; ok {
			g.AddVertex(v)
			name, _ := v.FindProperty("name")
			age, _ := v.FindProperty("age")
			rows = append(rows, []string{v.ID, strings.Join(v.Tags, ", "), name, age})
		}
	}
	return &model.QueryResult{
		Query:   stmt,
		Space:   d.space,
		Columns: []string{"vid", "tags", "name", "age"},
		Rows:    rows,
		Graph:   g,
	}
}

func (d *DemoClient) execLookup(stmt string, sp *demoSpace) *model.QueryResult {
	limit := extractLimit(stmt, 25)
	upper := strings.ToUpper(stmt)
	target := ""
	if idx := strings.Index(upper, " ON "); idx >= 0 {
		rest := strings.TrimSpace(stmt[idx+4:])
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			target = strings.Trim(fields[0], "`")
		}
	}
	g := model.NewGraphData()
	var rows [][]string
	for _, id := range sp.graph.VertexOrder {
		v := sp.graph.Vertices[id]
		if target != "" {
			has := false
			for _, t := range v.Tags {
				if strings.EqualFold(t, target) {
					has = true
					break
				}
			}
			if !has {
				continue
			}
		}
		g.AddVertex(v)
		rows = append(rows, []string{v.ID, strings.Join(v.Tags, ", "), v.DisplayLabel("name")})
		if len(rows) >= limit {
			break
		}
	}
	return &model.QueryResult{
		Query:   stmt,
		Space:   d.space,
		Columns: []string{"VertexID", "Tags", "Name"},
		Rows:    rows,
		Graph:   g,
	}
}

func (d *DemoClient) execFindPath(stmt string, sp *demoSpace) *model.QueryResult {
	ids := extractQuotedStrings(stmt)
	if len(ids) < 2 {
		return errorResult(stmt, d.space, `FIND SHORTEST PATH requires FROM "<src>" TO "<dst>"`)
	}
	src, dst := ids[0], ids[1]
	bidirect := strings.Contains(strings.ToUpper(stmt), "BIDIRECT")

	edgeFilter := map[string]bool{}
	for _, es := range sp.schema.Edges {
		if strings.Contains(strings.ToLower(stmt), strings.ToLower(es.Name)) {
			edgeFilter[strings.ToLower(es.Name)] = true
		}
	}

	type prevStep struct {
		from string
		edge *model.Edge
	}
	visited := map[string]bool{src: true}
	parent := map[string]prevStep{}
	queue := []string{src}
	found := false

	for len(queue) > 0 && !found {
		curr := queue[0]
		queue = queue[1:]
		for _, eid := range sp.graph.EdgeOrder {
			e := sp.graph.Edges[eid]
			if len(edgeFilter) > 0 && !edgeFilter[strings.ToLower(e.Name)] {
				continue
			}
			var next string
			if e.SrcID == curr {
				next = e.DstID
			} else if bidirect && e.DstID == curr {
				next = e.SrcID
			} else {
				continue
			}
			if visited[next] {
				continue
			}
			visited[next] = true
			parent[next] = prevStep{from: curr, edge: e}
			if next == dst {
				found = true
				break
			}
			queue = append(queue, next)
		}
	}

	g := model.NewGraphData()
	if !found {
		return &model.QueryResult{
			Query:   stmt,
			Space:   d.space,
			Columns: []string{"path"},
			Rows:    nil,
			Graph:   g,
			Comment: fmt.Sprintf("No path found between %s and %s", src, dst),
		}
	}

	var edges []*model.Edge
	var nodes []string
	cur := dst
	for cur != src {
		nodes = append(nodes, cur)
		st := parent[cur]
		edges = append(edges, st.edge)
		cur = st.from
	}
	nodes = append(nodes, src)
	// Reverse
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	for i, j := 0, len(edges)-1; i < j; i, j = i+1, j-1 {
		edges[i], edges[j] = edges[j], edges[i]
	}

	var pathParts []string
	for i, nid := range nodes {
		v := sp.graph.Vertices[nid]
		g.AddVertex(v)
		pathParts = append(pathParts, fmt.Sprintf("(%s:%s)", nid, v.DisplayLabel("name")))
		if i < len(edges) {
			g.AddEdge(edges[i])
			pathParts = append(pathParts, fmt.Sprintf("-[:%s]->", edges[i].Name))
		}
	}
	return &model.QueryResult{
		Query:   stmt,
		Space:   d.space,
		Columns: []string{"hops", "path"},
		Rows:    [][]string{{strconv.Itoa(len(edges)), strings.Join(pathParts, "")}},
		Graph:   g,
	}
}

func tableResult(stmt, space string, cols []string, rows [][]string) *model.QueryResult {
	return &model.QueryResult{
		Query:   stmt,
		Space:   space,
		Columns: cols,
		Rows:    rows,
		Graph:   model.NewGraphData(),
	}
}

func errorResult(stmt, space, msg string) *model.QueryResult {
	return &model.QueryResult{
		Query: stmt,
		Space: space,
		Error: msg,
		Graph: model.NewGraphData(),
	}
}

func formatPropMap(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+m[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func buildResultJSON(qr *model.QueryResult) string {
	rows := make([]map[string]string, 0, len(qr.Rows))
	for _, r := range qr.Rows {
		obj := make(map[string]string, len(qr.Columns))
		for i, c := range qr.Columns {
			if i < len(r) {
				obj[c] = r[i]
			}
		}
		rows = append(rows, obj)
	}
	payload := map[string]any{
		"space":      qr.Space,
		"latency_ms": qr.LatencyMs,
		"columns":    qr.Columns,
		"row_count":  len(qr.Rows),
		"rows":       rows,
	}
	if qr.HasGraph() {
		payload["graph_summary"] = map[string]int{
			"vertices": len(qr.Graph.Vertices),
			"edges":    len(qr.Graph.Edges),
		}
	}
	if len(qr.Plan) > 0 {
		payload["plan"] = qr.Plan
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return string(b)
}

func buildDemoPlan(query string, rowCount int, profile bool) []model.PlanNode {
	prof := func(rows, us int) string {
		if !profile {
			return ""
		}
		return fmt.Sprintf("rows=%d exec=%dus total=%dus", rows, us, us+40)
	}
	return []model.PlanNode{
		{ID: 5, Name: "Project", Dependencies: []int64{4}, OutputVar: "__Project_5", Description: "columns: [p]", ProfilingStats: prof(rowCount, 120)},
		{ID: 4, Name: "Limit", Dependencies: []int64{3}, OutputVar: "__Limit_4", Description: fmt.Sprintf("count: %d", max(rowCount, 15)), ProfilingStats: prof(rowCount, 35)},
		{ID: 3, Name: "Traverse", Dependencies: []int64{2}, OutputVar: "__Traverse_3", Description: "edgeTypes: [follow, serve], steps: 1..1", ProfilingStats: prof(rowCount*2, 410)},
		{ID: 2, Name: "Filter", Dependencies: []int64{1}, OutputVar: "__Filter_2", Description: "condition: " + truncate(query, 48), ProfilingStats: prof(rowCount+4, 95)},
		{ID: 1, Name: "IndexScan", Dependencies: []int64{0}, OutputVar: "__IndexScan_1", Description: "index: player_name_index", ProfilingStats: prof(20, 260)},
		{ID: 0, Name: "Start", Dependencies: nil, OutputVar: "__Start_0", Description: "entry", ProfilingStats: prof(1, 5)},
	}
}

func truncate(s string, maxLen int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func stripComments(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "--") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func splitStatements(src string) []string {
	var out []string
	var cur strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
		} else if ch == '"' && !inSingle {
			inDouble = !inDouble
		} else if ch == ';' && !inSingle && !inDouble {
			if s := strings.TrimSpace(cur.String()); s != "" {
				out = append(out, s)
			}
			cur.Reset()
			continue
		}
		cur.WriteByte(ch)
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
}

func lastIdent(stmt string) string {
	fields := strings.Fields(stmt)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[len(fields)-1], "`\"';")
}

// buildBasketballSpace constructs the rich NebulaGraph basketballplayer dataset.
func buildBasketballSpace() *demoSpace {
	g := model.NewGraphData()

	addPlayer := func(id, name string, age int, pos, university string) {
		tags := []string{"player"}
		props := map[string]map[string]string{
			"player": {
				"name":     name,
				"age":      strconv.Itoa(age),
				"position": pos,
			},
		}
		if university != "" {
			tags = append(tags, "bachelor")
			props["bachelor"] = map[string]string{
				"name":       name,
				"speciality": university,
			}
		}
		g.AddVertex(&model.Vertex{ID: id, Tags: tags, Properties: props})
	}
	addTeam := func(id, name, city, conf string) {
		g.AddVertex(&model.Vertex{
			ID:   id,
			Tags: []string{"team"},
			Properties: map[string]map[string]string{
				"team": {"name": name, "city": city, "conference": conf},
			},
		})
	}
	addCoach := func(id, name string, titles int) {
		g.AddVertex(&model.Vertex{
			ID:   id,
			Tags: []string{"coach"},
			Properties: map[string]map[string]string{
				"coach": {"name": name, "championships": strconv.Itoa(titles)},
			},
		})
	}
	addFollow := func(src, dst string, degree int) {
		g.AddEdge(&model.Edge{
			SrcID: src, DstID: dst, Name: "follow",
			Properties: map[string]string{"degree": strconv.Itoa(degree)},
		})
	}
	addServe := func(src, dst string, rank int64, startY, endY int) {
		g.AddEdge(&model.Edge{
			SrcID: src, DstID: dst, Name: "serve", Rank: rank,
			Properties: map[string]string{
				"start_year": strconv.Itoa(startY),
				"end_year":   strconv.Itoa(endY),
			},
		})
	}
	addCoachEdge := func(coach, team string, since int) {
		g.AddEdge(&model.Edge{
			SrcID: coach, DstID: team, Name: "coaches",
			Properties: map[string]string{"since": strconv.Itoa(since)},
		})
	}

	// Players (classic NebulaGraph VIDs)
	addPlayer("player100", "Tim Duncan", 42, "PF/C", "Wake Forest - Psychology")
	addPlayer("player101", "Tony Parker", 36, "PG", "")
	addPlayer("player102", "LaMarcus Aldridge", 33, "PF", "Texas")
	addPlayer("player103", "Rudy Gay", 32, "SF", "UConn")
	addPlayer("player104", "Marco Belinelli", 32, "SG", "")
	addPlayer("player105", "Danny Green", 31, "SG", "North Carolina")
	addPlayer("player106", "Kyle Anderson", 25, "SF", "UCLA")
	addPlayer("player107", "Aron Baynes", 32, "C", "Washington State")
	addPlayer("player108", "Boris Diaw", 36, "PF", "")
	addPlayer("player109", "Tiago Splitter", 34, "C", "")
	addPlayer("player110", "Cory Joseph", 27, "PG", "Texas")
	addPlayer("player111", "David West", 38, "PF", "Xavier")
	addPlayer("player113", "Yao Ming", 38, "C", "Shanghai Jiao Tong")
	addPlayer("player114", "Tracy McGrady", 39, "SG/SF", "")
	addPlayer("player115", "Kobe Bryant", 40, "SG", "")
	addPlayer("player116", "Shaquille O'Neal", 47, "C", "LSU")
	addPlayer("player118", "Manu Ginobili", 41, "SG", "")
	addPlayer("player121", "Stephen Curry", 31, "PG", "Davidson")
	addPlayer("player122", "Klay Thompson", 29, "SG", "Washington State")
	addPlayer("player125", "LeBron James", 34, "SF/PF", "")

	// Teams
	addTeam("team200", "Warriors", "San Francisco", "West")
	addTeam("team201", "Nuggets", "Denver", "West")
	addTeam("team202", "Rockets", "Houston", "West")
	addTeam("team203", "Trail Blazers", "Portland", "West")
	addTeam("team204", "Spurs", "San Antonio", "West")
	addTeam("team205", "Lakers", "Los Angeles", "West")
	addTeam("team206", "Cavaliers", "Cleveland", "East")

	// Coaches
	addCoach("coach300", "Gregg Popovich", 5)
	addCoach("coach301", "Steve Kerr", 4)
	addCoach("coach302", "Phil Jackson", 11)

	// Interleaved core relationships so LIMIT 25..30 showcases heterogeneous tags
	// (player, bachelor, team, coach) and parallel/bidirectional curved edges right away!
	addCoachEdge("coach300", "team204", 1996)
	addCoachEdge("coach301", "team200", 2014)
	addCoachEdge("coach302", "team205", 1999)

	// Spurs core + bidirectional follows
	addServe("player100", "team204", 0, 1997, 2016)
	addServe("player101", "team204", 0, 2001, 2018)
	addServe("player118", "team204", 0, 2002, 2018)
	addFollow("player100", "player101", 95)
	addFollow("player101", "player100", 95)
	addFollow("player100", "player118", 98)
	addFollow("player118", "player100", 98)
	addFollow("player101", "player118", 92)

	// Warriors Splash Bros
	addServe("player121", "team200", 0, 2009, 2026)
	addServe("player122", "team200", 0, 2011, 2024)
	addFollow("player121", "player122", 97)
	addFollow("player122", "player121", 96)

	// Lakers & Cavaliers (including LeBron's two stints with Cavaliers using rank 0 & rank 1!)
	addServe("player115", "team205", 0, 1996, 2016)
	addServe("player116", "team205", 0, 1996, 2004)
	addServe("player125", "team205", 0, 2018, 2026)
	addServe("player125", "team206", 0, 2003, 2010)
	addServe("player125", "team206", 1, 2014, 2018)
	addFollow("player115", "player116", 85)
	addFollow("player116", "player115", 82)
	addFollow("player115", "player100", 90)
	addFollow("player125", "player115", 93)
	addFollow("player125", "player121", 86)
	addFollow("player121", "player125", 84)

	// Rockets Yao + T-Mac bridge
	addServe("player113", "team202", 0, 2002, 2011)
	addServe("player114", "team202", 0, 2004, 2010)
	addServe("player114", "team204", 0, 2013, 2013)
	addFollow("player113", "player114", 94)
	addFollow("player114", "player113", 92)
	addFollow("player114", "player115", 88)

	// Additional Spurs role players & cross-team serves (revealed via Expand!)
	addFollow("player101", "player102", 90)
	addFollow("player102", "player100", 75)
	addFollow("player103", "player102", 70)
	addFollow("player104", "player100", 80)
	addFollow("player105", "player100", 85)
	addFollow("player105", "player101", 82)
	addFollow("player106", "player100", 72)
	addFollow("player107", "player100", 68)
	addFollow("player108", "player101", 91)
	addFollow("player109", "player100", 80)
	addFollow("player110", "player101", 78)
	addFollow("player111", "player100", 84)

	addServe("player102", "team204", 0, 2015, 2021)
	addServe("player102", "team203", 0, 2006, 2015)
	addServe("player103", "team204", 0, 2017, 2021)
	addServe("player104", "team204", 0, 2013, 2015)
	addServe("player104", "team204", 1, 2018, 2020)
	addServe("player105", "team204", 0, 2010, 2018)
	addServe("player105", "team205", 0, 2019, 2020)
	addServe("player106", "team204", 0, 2014, 2018)
	addServe("player107", "team204", 0, 2013, 2015)
	addServe("player108", "team204", 0, 2012, 2016)
	addServe("player109", "team204", 0, 2010, 2015)
	addServe("player110", "team204", 0, 2011, 2015)
	addServe("player111", "team204", 0, 2015, 2016)
	addServe("player111", "team200", 0, 2016, 2018)

	schema := &model.SpaceSchema{
		Space:         "basketballplayer",
		VIDType:       "FIXED_STRING(32)",
		PartitionNum:  10,
		ReplicaFactor: 1,
		TotalVertices: int64(len(g.Vertices)),
		TotalEdges:    int64(len(g.Edges)),
		Tags: []model.TagSchema{
			{
				Name:  "player",
				Count: 20,
				Properties: []model.PropertySchema{
					{Field: "name", Type: "string", Null: "NO"},
					{Field: "age", Type: "int64", Null: "YES", Default: "0"},
					{Field: "position", Type: "string", Null: "YES"},
				},
			},
			{
				Name:  "team",
				Count: 7,
				Properties: []model.PropertySchema{
					{Field: "name", Type: "string", Null: "NO"},
					{Field: "city", Type: "string", Null: "YES"},
					{Field: "conference", Type: "string", Null: "YES"},
				},
			},
			{
				Name:  "bachelor",
				Count: 11,
				Properties: []model.PropertySchema{
					{Field: "name", Type: "string", Null: "NO"},
					{Field: "speciality", Type: "string", Null: "YES"},
				},
			},
			{
				Name:  "coach",
				Count: 3,
				Properties: []model.PropertySchema{
					{Field: "name", Type: "string", Null: "NO"},
					{Field: "championships", Type: "int64", Null: "YES", Default: "0"},
				},
			},
		},
		Edges: []model.EdgeSchema{
			{
				Name:  "follow",
				Count: 28,
				Properties: []model.PropertySchema{
					{Field: "degree", Type: "int64", Null: "YES", Default: "0"},
				},
			},
			{
				Name:  "serve",
				Count: 27,
				Properties: []model.PropertySchema{
					{Field: "start_year", Type: "int64", Null: "YES"},
					{Field: "end_year", Type: "int64", Null: "YES"},
				},
			},
			{
				Name:  "coaches",
				Count: 3,
				Properties: []model.PropertySchema{
					{Field: "since", Type: "int64", Null: "YES"},
				},
			},
		},
		Indexes: []model.IndexSchema{
			{Name: "player_name_index", Kind: "TAG", Target: "player", Fields: []string{"name(20)"}},
			{Name: "player_age_index", Kind: "TAG", Target: "player", Fields: []string{"age"}},
			{Name: "team_name_index", Kind: "TAG", Target: "team", Fields: []string{"name(20)"}},
			{Name: "follow_degree_index", Kind: "EDGE", Target: "follow", Fields: []string{"degree"}},
		},
	}

	return &demoSpace{name: "basketballplayer", schema: schema, graph: g}
}

// buildTechEcosystemSpace provides a second space so Space Switching can be tested immediately.
func buildTechEcosystemSpace() *demoSpace {
	g := model.NewGraphData()
	g.AddVertex(&model.Vertex{ID: "nebula", Tags: []string{"project"}, Properties: map[string]map[string]string{"project": {"name": "NebulaGraph", "lang": "C++", "category": "GraphDB"}}})
	g.AddVertex(&model.Vertex{ID: "mygo", Tags: []string{"project"}, Properties: map[string]map[string]string{"project": {"name": "MyGo", "lang": "Go", "category": "Desktop UI"}}})
	g.AddVertex(&model.Vertex{ID: "nebuladesk", Tags: []string{"project"}, Properties: map[string]map[string]string{"project": {"name": "NebulaDesk", "lang": "Go", "category": "Native Client"}}})
	g.AddVertex(&model.Vertex{ID: "nebulago", Tags: []string{"library"}, Properties: map[string]map[string]string{"library": {"name": "nebula-go v3", "lang": "Go"}}})
	g.AddVertex(&model.Vertex{ID: "metal", Tags: []string{"gpu_api"}, Properties: map[string]map[string]string{"gpu_api": {"name": "Apple Metal", "platform": "macOS"}}})
	g.AddVertex(&model.Vertex{ID: "d3d11", Tags: []string{"gpu_api"}, Properties: map[string]map[string]string{"gpu_api": {"name": "Direct3D 11", "platform": "Windows"}}})

	g.AddEdge(&model.Edge{SrcID: "nebuladesk", DstID: "mygo", Name: "built_with", Properties: map[string]string{"version": "v0.10.0"}})
	g.AddEdge(&model.Edge{SrcID: "nebuladesk", DstID: "nebulago", Name: "depends_on", Properties: map[string]string{"version": "v3.8.0"}})
	g.AddEdge(&model.Edge{SrcID: "nebulago", DstID: "nebula", Name: "connects_to", Properties: map[string]string{"protocol": "Thrift RPC"}})
	g.AddEdge(&model.Edge{SrcID: "mygo", DstID: "metal", Name: "renders_via", Properties: map[string]string{"cgo": "false"}})
	g.AddEdge(&model.Edge{SrcID: "mygo", DstID: "d3d11", Name: "renders_via", Properties: map[string]string{"cgo": "false"}})

	schema := &model.SpaceSchema{
		Space:         "tech_ecosystem",
		VIDType:       "FIXED_STRING(32)",
		PartitionNum:  4,
		ReplicaFactor: 1,
		TotalVertices: int64(len(g.Vertices)),
		TotalEdges:    int64(len(g.Edges)),
		Tags: []model.TagSchema{
			{Name: "project", Count: 3, Properties: []model.PropertySchema{{Field: "name", Type: "string"}, {Field: "lang", Type: "string"}, {Field: "category", Type: "string"}}},
			{Name: "library", Count: 1, Properties: []model.PropertySchema{{Field: "name", Type: "string"}, {Field: "lang", Type: "string"}}},
			{Name: "gpu_api", Count: 2, Properties: []model.PropertySchema{{Field: "name", Type: "string"}, {Field: "platform", Type: "string"}}},
		},
		Edges: []model.EdgeSchema{
			{Name: "built_with", Count: 1, Properties: []model.PropertySchema{{Field: "version", Type: "string"}}},
			{Name: "depends_on", Count: 1, Properties: []model.PropertySchema{{Field: "version", Type: "string"}}},
			{Name: "connects_to", Count: 1, Properties: []model.PropertySchema{{Field: "protocol", Type: "string"}}},
			{Name: "renders_via", Count: 2, Properties: []model.PropertySchema{{Field: "cgo", Type: "string"}}},
		},
	}
	return &demoSpace{name: "tech_ecosystem", schema: schema, graph: g}
}
