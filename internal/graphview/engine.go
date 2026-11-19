package graphview

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"nebula_client/internal/model"
)

// LayoutMode chooses how nodes are positioned on the 2D graph canvas.
type LayoutMode int

const (
	LayoutForce LayoutMode = iota
	LayoutCircular
	LayoutHierarchical
	LayoutGrid
)

func (m LayoutMode) String() string {
	switch m {
	case LayoutCircular:
		return "Circle"
	case LayoutHierarchical:
		return "Tree"
	case LayoutGrid:
		return "Grid"
	default:
		return "Force"
	}
}

// NodeVisual stores 2D physics & rendering state for a single graph vertex.
type NodeVisual struct {
	ID     string
	Vertex *model.Vertex
	X, Y   float32
	VX, VY float32
	Radius float32
	Pinned bool
	Degree int
}

// EdgeCurve caches geometry parameters for rendering & hit-testing an edge.
type EdgeCurve struct {
	Edge      *model.Edge
	IsLoop    bool
	Curve     float32 // perpendicular Bézier offset in world units (0 = straight line)
	LoopAngle float32
}

// State holds all visual, camera, selection, and physics state for one GraphView instance.
type State struct {
	Graph *model.GraphData

	Nodes     map[string]*NodeVisual
	NodeOrder []string
	Curves    []EdgeCurve

	// Camera in world coordinates: screen = center + (world - pan) * zoom
	PanX, PanY float32
	Zoom       float32
	NeedsFit   bool

	// Layout & simulation
	Layout     LayoutMode
	Simulating bool
	Energy     float32
	SimSteps   int

	// Display options
	ShowEdgeLabels bool
	LabelProperty  string // "" = auto (name/title/vid), "__id__" = always VID
	DimUnrelated   bool
	ShowMinimap    bool

	// Selection & hover
	SelectedNodeID string
	SelectedEdgeID string
	HoveredNodeID  string
	HoveredEdgeID  string
	ContextNodeID  string

	// Filtering & search
	SearchQuery     string
	HiddenTags      map[string]bool
	HiddenEdgeTypes map[string]bool
	HiddenNodes     map[string]bool

	// Drag state
	DragMode       int // 0=none, 1=pan, 2=node
	DragNodeID     string
	DragStartX     float32
	DragStartY     float32
	LastPointerX   float32
	LastPointerY   float32
	DragMoved      bool
	LastClickID    string
	LastClickAt    int64 // unix milli for double-click detection on nodes
	LastCanvasRect ui.Rect

	// Tag color registry
	tagColors  map[string]ui.Color
	edgeColors map[string]ui.Color

	// Cached summaries updated only when graph topology changes (zero per-frame sorting)
	CachedTagCounts  []model.NameCount
	CachedEdgeCounts []model.NameCount
	CachedTopHubs    []HubSummary
}

// HubSummary holds pre-sorted top-degree vertex metadata for the Inspector panel.
type HubSummary struct {
	ID     string
	Tag    string
	Degree int
}

var tagPalette = []ui.Color{
	ui.Hex("#6366f1"), // Indigo
	ui.Hex("#10b981"), // Emerald
	ui.Hex("#f59e0b"), // Amber
	ui.Hex("#ec4899"), // Pink
	ui.Hex("#06b6d4"), // Cyan
	ui.Hex("#8b5cf6"), // Violet
	ui.Hex("#f97316"), // Orange
	ui.Hex("#14b8a6"), // Teal
	ui.Hex("#ef4444"), // Red
	ui.Hex("#84cc16"), // Lime
}

var edgePalette = []ui.Color{
	ui.Hex("#64748b"), // Slate
	ui.Hex("#38bdf8"), // Sky
	ui.Hex("#a78bfa"), // Violet
	ui.Hex("#34d399"), // Emerald
	ui.Hex("#fbbf24"), // Amber
	ui.Hex("#fb7185"), // Rose
}

// NewState initializes a fresh GraphView state.
func NewState() *State {
	return &State{
		Graph:           model.NewGraphData(),
		Nodes:           make(map[string]*NodeVisual),
		Zoom:            1.0,
		Layout:          LayoutForce,
		ShowEdgeLabels:  true,
		DimUnrelated:    true,
		ShowMinimap:     true,
		HiddenTags:      make(map[string]bool),
		HiddenEdgeTypes: make(map[string]bool),
		HiddenNodes:     make(map[string]bool),
		tagColors:       make(map[string]ui.Color),
		edgeColors:      make(map[string]ui.Color),
	}
}

// SetGraph replaces the visualized graph and computes an initial layout.
func (s *State) SetGraph(g *model.GraphData) {
	if g == nil {
		g = model.NewGraphData()
	}
	s.Graph = g
	s.Nodes = make(map[string]*NodeVisual, len(g.Vertices))
	s.NodeOrder = nil
	s.SelectedNodeID = ""
	s.SelectedEdgeID = ""
	s.HoveredNodeID = ""
	s.HoveredEdgeID = ""
	s.HiddenTags = make(map[string]bool)
	s.HiddenEdgeTypes = make(map[string]bool)
	s.HiddenNodes = make(map[string]bool)

	s.syncNodes(true)
	s.ApplyLayout(s.Layout)
	s.NeedsFit = true
}

// MergeGraph smoothly adds newly expanded vertices/edges (e.g. from double-click expansion)
// around the anchor vertex while preserving existing node positions.
func (s *State) MergeGraph(other *model.GraphData, anchorID string) (addedV, addedE int) {
	if other == nil || other.Empty() {
		return 0, 0
	}
	if s.Graph == nil {
		s.Graph = model.NewGraphData()
	}
	addedV, addedE = s.Graph.Merge(other)
	if addedV == 0 && addedE == 0 {
		return 0, 0
	}

	var ax, ay float32
	if anchor, ok := s.Nodes[anchorID]; ok {
		ax, ay = anchor.X, anchor.Y
	}

	// Place only newly added nodes in a tidy ring around the anchor node
	newIdx := 0
	for _, id := range s.Graph.VertexOrder {
		v := s.Graph.Vertices[id]
		if existing, ok := s.Nodes[id]; ok {
			existing.Vertex = v
			continue
		}
		angle := float64(newIdx) * 2.399963229728653 // golden angle
		dist := 110.0 + float64(newIdx%4)*24.0
		nv := &NodeVisual{
			ID:     id,
			Vertex: v,
			X:      ax + float32(math.Cos(angle)*dist),
			Y:      ay + float32(math.Sin(angle)*dist),
			Radius: 22,
		}
		s.Nodes[id] = nv
		s.NodeOrder = append(s.NodeOrder, id)
		newIdx++
	}

	s.recomputeDegreesAndCurves()
	// Run a brief local settling burst if in Force mode
	if s.Layout == LayoutForce {
		s.Simulating = true
		s.Energy = 1.0
		s.SimSteps = 0
	}
	return addedV, addedE
}

func (s *State) syncNodes(resetPositions bool) {
	s.NodeOrder = s.NodeOrder[:0]
	n := len(s.Graph.VertexOrder)
	radius := float32(120 + n*5)
	if radius > 340 {
		radius = 340
	}
	for i, id := range s.Graph.VertexOrder {
		v := s.Graph.Vertices[id]
		nv, ok := s.Nodes[id]
		if !ok || resetPositions {
			angle := 2 * math.Pi * float64(i) / float64(max(n, 1))
			// Slight deterministic radial jitter prevents exact collinearity in force sim
			r := float64(radius) * (0.78 + 0.22*math.Sin(float64(i)*1.618))
			nv = &NodeVisual{
				ID:     id,
				Vertex: v,
				X:      float32(math.Cos(angle) * r),
				Y:      float32(math.Sin(angle) * r),
				Radius: 22,
			}
			s.Nodes[id] = nv
		} else {
			nv.Vertex = v
		}
		s.NodeOrder = append(s.NodeOrder, id)
	}
	s.recomputeDegreesAndCurves()
}

func (s *State) recomputeDegreesAndCurves() {
	for _, nv := range s.Nodes {
		nv.Degree = 0
	}
	// Group parallel edges between the same unordered pair so we can fan out Bézier curves cleanly
	type pairKey struct{ a, b string }
	groups := make(map[pairKey][]*model.Edge)
	var groupOrder []pairKey

	for _, eid := range s.Graph.EdgeOrder {
		e := s.Graph.Edges[eid]
		if nv, ok := s.Nodes[e.SrcID]; ok {
			nv.Degree++
		}
		if nv, ok := s.Nodes[e.DstID]; ok {
			nv.Degree++
		}
		pk := pairKey{e.SrcID, e.DstID}
		if pk.a > pk.b {
			pk.a, pk.b = pk.b, pk.a
		}
		if _, exists := groups[pk]; !exists {
			groupOrder = append(groupOrder, pk)
		}
		groups[pk] = append(groups[pk], e)
	}

	// Scale node radii gently by degree (18..30 DIPs)
	for _, nv := range s.Nodes {
		r := float32(19) + float32(math.Sqrt(float64(nv.Degree)))*2.4
		if r > 32 {
			r = 32
		}
		nv.Radius = r
	}

	s.Curves = s.Curves[:0]
	for _, pk := range groupOrder {
		edges := groups[pk]
		m := len(edges)
		for idx, e := range edges {
			if e.SrcID == e.DstID {
				s.Curves = append(s.Curves, EdgeCurve{
					Edge:      e,
					IsLoop:    true,
					LoopAngle: -math.Pi/2 + float32(idx)*0.6,
				})
				continue
			}
			var curve float32
			if m > 1 {
				// Spread symmetric offsets: e.g. for 2 edges -> -28, +28; for 3 -> -36, 0, +36
				step := float32(34)
				offset := (float32(idx) - float32(m-1)/2.0) * step
				// Keep direction consistent relative to canonical pair orientation
				if e.SrcID != pk.a {
					offset = -offset
				}
				curve = offset
			}
			s.Curves = append(s.Curves, EdgeCurve{
				Edge:  e,
				Curve: curve,
			})
		}
	}

	// Precompute legend counts and top-degree hubs once per topology change
	s.CachedTagCounts = s.Graph.TagCounts()
	s.CachedEdgeCounts = s.Graph.EdgeTypeCounts()
	hubs := make([]HubSummary, 0, len(s.NodeOrder))
	for _, id := range s.NodeOrder {
		nv := s.Nodes[id]
		hubs = append(hubs, HubSummary{
			ID:     id,
			Tag:    nv.Vertex.PrimaryTag(),
			Degree: nv.Degree,
		})
	}
	sort.Slice(hubs, func(i, j int) bool {
		if hubs[i].Degree != hubs[j].Degree {
			return hubs[i].Degree > hubs[j].Degree
		}
		return hubs[i].ID < hubs[j].ID
	})
	if len(hubs) > 12 {
		hubs = hubs[:12]
	}
	s.CachedTopHubs = hubs
}

// ApplyLayout arranges nodes using the chosen layout algorithm.
func (s *State) ApplyLayout(mode LayoutMode) {
	s.Layout = mode
	n := len(s.NodeOrder)
	if n == 0 {
		s.Simulating = false
		return
	}

	for _, nv := range s.Nodes {
		nv.VX, nv.VY = 0, 0
	}

	switch mode {
	case LayoutCircular:
		s.layoutCircular()
		s.Simulating = false
	case LayoutHierarchical:
		s.layoutHierarchical()
		s.Simulating = false
	case LayoutGrid:
		s.layoutGrid()
		s.Simulating = false
	default:
		// Force-directed: pre-warm within a strict 18ms UI-thread budget so large graphs
		// (500..2000+ nodes) NEVER freeze the window, then smoothly finish settling at 60fps.
		s.layoutCircular()
		s.Simulating = true
		s.Energy = 1.0
		s.SimSteps = 0
		deadline := time.Now().Add(18 * time.Millisecond)
		for i := 0; i < 160 && s.Simulating; i++ {
			s.StepForce(1.0)
			if i >= 12 && i%4 == 0 && time.Now().After(deadline) {
				break
			}
		}
	}
}

func (s *State) layoutCircular() {
	// Sort nodes by PrimaryTag then ID so same-tag vertices cluster cleanly
	ids := append([]string(nil), s.NodeOrder...)
	sort.Slice(ids, func(i, j int) bool {
		vi, vj := s.Nodes[ids[i]].Vertex, s.Nodes[ids[j]].Vertex
		if vi.PrimaryTag() != vj.PrimaryTag() {
			return vi.PrimaryTag() < vj.PrimaryTag()
		}
		return vi.ID < vj.ID
	})
	n := len(ids)
	radius := float32(math.Max(140, float64(n)*18))
	if radius > 420 {
		radius = 420
	}
	for i, id := range ids {
		nv := s.Nodes[id]
		if nv.Pinned {
			continue
		}
		angle := -math.Pi/2 + 2*math.Pi*float64(i)/float64(max(n, 1))
		nv.X = float32(math.Cos(angle) * float64(radius))
		nv.Y = float32(math.Sin(angle) * float64(radius))
	}
}

func (s *State) layoutHierarchical() {
	// Pick root: selected node if present, otherwise highest-degree node
	root := s.SelectedNodeID
	if root == "" || s.Nodes[root] == nil {
		bestDeg := -1
		for _, id := range s.NodeOrder {
			if s.Nodes[id].Degree > bestDeg {
				bestDeg = s.Nodes[id].Degree
				root = id
			}
		}
	}
	// Build undirected adjacency
	adj := make(map[string][]string, len(s.NodeOrder))
	for _, c := range s.Curves {
		e := c.Edge
		adj[e.SrcID] = append(adj[e.SrcID], e.DstID)
		adj[e.DstID] = append(adj[e.DstID], e.SrcID)
	}

	levels := make(map[int][]string)
	visited := make(map[string]bool)

	bfsFrom := func(start string, baseLevel int) int {
		type item struct {
			id  string
			lvl int
		}
		q := []item{{start, baseLevel}}
		visited[start] = true
		maxLvl := baseLevel
		for len(q) > 0 {
			cur := q[0]
			q = q[1:]
			levels[cur.lvl] = append(levels[cur.lvl], cur.id)
			if cur.lvl > maxLvl {
				maxLvl = cur.lvl
			}
			for _, nb := range adj[cur.id] {
				if !visited[nb] {
					visited[nb] = true
					q = append(q, item{nb, cur.lvl + 1})
				}
			}
		}
		return maxLvl
	}

	maxLvl := 0
	if root != "" {
		maxLvl = bfsFrom(root, 0)
	}
	for _, id := range s.NodeOrder {
		if !visited[id] {
			maxLvl = bfsFrom(id, maxLvl+1)
		}
	}

	const rowGap = float32(120)
	const colGap = float32(130)
	totalH := float32(maxLvl) * rowGap
	for lvl := 0; lvl <= maxLvl; lvl++ {
		row := levels[lvl]
		rowW := float32(len(row)-1) * colGap
		for idx, id := range row {
			nv := s.Nodes[id]
			if nv.Pinned {
				continue
			}
			nv.X = -rowW/2 + float32(idx)*colGap
			nv.Y = -totalH/2 + float32(lvl)*rowGap
		}
	}
}

func (s *State) layoutGrid() {
	n := len(s.NodeOrder)
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	if cols < 1 {
		cols = 1
	}
	rows := (n + cols - 1) / cols
	const spacing = float32(125)
	for i, id := range s.NodeOrder {
		nv := s.Nodes[id]
		if nv.Pinned {
			continue
		}
		r := i / cols
		c := i % cols
		nv.X = (float32(c) - float32(cols-1)/2) * spacing
		nv.Y = (float32(r) - float32(rows-1)/2) * spacing
	}
}

// StepForce executes one iteration of Fruchterman-Reingold force-directed layout.
// Returns true while the graph is still visibly moving.
func (s *State) StepForce(dt float32) bool {
	if !s.Simulating || len(s.NodeOrder) == 0 {
		return false
	}
	if dt <= 0 || dt > 2.0 {
		dt = 1.0
	}

	const (
		repulsion   = float32(28000.0)
		springK     = float32(0.020)
		springLen   = float32(185.0)
		gravity     = float32(0.008)
		damping     = float32(0.82)
		maxVelocity = float32(24.0)
	)

	n := len(s.NodeOrder)
	nodes := make([]*NodeVisual, n)
	for i, id := range s.NodeOrder {
		nodes[i] = s.Nodes[id]
	}

	// Pairwise repulsion: exact O(N^2) for small graphs (<= 120 nodes),
	// 2D Spatial Hash Grid O(N) for larger graphs so 500..2000+ nodes stay interactive!
	applyRepulsionPair := func(ni, nj *NodeVisual, idxI, idxJ int) {
		dx := ni.X - nj.X
		dy := ni.Y - nj.Y
		distSq := dx*dx + dy*dy
		if distSq < 25 {
			dx = float32(idxI%3-1) + 0.5
			dy = float32(idxJ%3-1) - 0.5
			distSq = dx*dx + dy*dy + 16
		}
		dist := float32(math.Sqrt(float64(distSq)))
		force := repulsion / distSq
		minSep := ni.Radius + nj.Radius + 78
		if dist < minSep {
			force += (minSep - dist) * 0.28
		}
		if force > 55 {
			force = 55
		}
		fx := (dx / dist) * force
		fy := (dy / dist) * force
		if !ni.Pinned {
			ni.VX += fx * dt
			ni.VY += fy * dt
		}
		if !nj.Pinned {
			nj.VX -= fx * dt
			nj.VY -= fy * dt
		}
	}

	if n <= 120 {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				applyRepulsionPair(nodes[i], nodes[j], i, j)
			}
		}
	} else {
		const cellSize = float32(420.0)
		type cellKey struct{ cx, cy int }
		grid := make(map[cellKey][]int, n)
		for i, nv := range nodes {
			ck := cellKey{
				cx: int(math.Floor(float64(nv.X / cellSize))),
				cy: int(math.Floor(float64(nv.Y / cellSize))),
			}
			grid[ck] = append(grid[ck], i)
		}
		// Only check pairs inside the same cell and 4 forward neighbor cells to avoid duplicates
		neighborOffsets := [][2]int{{1, 0}, {-1, 1}, {0, 1}, {1, 1}}
		for ck, bucket := range grid {
			for a := 0; a < len(bucket); a++ {
				i := bucket[a]
				for b := a + 1; b < len(bucket); b++ {
					j := bucket[b]
					applyRepulsionPair(nodes[i], nodes[j], i, j)
				}
			}
			for _, off := range neighborOffsets {
				other := grid[cellKey{ck.cx + off[0], ck.cy + off[1]}]
				if len(other) == 0 {
					continue
				}
				for _, i := range bucket {
					for _, j := range other {
						applyRepulsionPair(nodes[i], nodes[j], i, j)
					}
				}
			}
		}
	}

	// Spring attraction along edges
	for _, c := range s.Curves {
		e := c.Edge
		if e.SrcID == e.DstID {
			continue
		}
		ns := s.Nodes[e.SrcID]
		nd := s.Nodes[e.DstID]
		if ns == nil || nd == nil {
			continue
		}
		dx := nd.X - ns.X
		dy := nd.Y - ns.Y
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if dist < 1 {
			dist = 1
		}
		// Slightly longer rest length for hub nodes so labels have breathing room
		target := springLen + (ns.Radius+nd.Radius)*0.7
		delta := (dist - target) * springK
		fx := (dx / dist) * delta
		fy := (dy / dist) * delta
		if !ns.Pinned {
			ns.VX += fx * dt
			ns.VY += fy * dt
		}
		if !nd.Pinned {
			nd.VX -= fx * dt
			nd.VY -= fy * dt
		}
	}

	// Center gravity + integration
	var totalMove float32
	for _, nv := range nodes {
		if nv.Pinned || (s.DragMode == 2 && s.DragNodeID == nv.ID) {
			nv.VX, nv.VY = 0, 0
			continue
		}
		nv.VX -= nv.X * gravity * dt
		nv.VY -= nv.Y * gravity * dt
		nv.VX *= damping
		nv.VY *= damping

		speed := float32(math.Sqrt(float64(nv.VX*nv.VX + nv.VY*nv.VY)))
		if speed > maxVelocity {
			scale := maxVelocity / speed
			nv.VX *= scale
			nv.VY *= scale
			speed = maxVelocity
		}
		nv.X += nv.VX * dt
		nv.Y += nv.VY * dt
		totalMove += speed
	}

	s.SimSteps++
	s.Energy = totalMove / float32(max(n, 1))
	maxSteps := 360
	if n > 200 {
		maxSteps = 180
	}
	if s.Energy < 0.16 || s.SimSteps >= maxSteps {
		s.Simulating = false
	}
	return s.Simulating
}

// WorldToScreen maps world coordinates to DIP coordinates inside rect.
func (s *State) WorldToScreen(wx, wy float32, rect ui.Rect) (sx, sy float32) {
	cx := rect.X + rect.W/2
	cy := rect.Y + rect.H/2
	return cx + (wx-s.PanX)*s.Zoom, cy + (wy-s.PanY)*s.Zoom
}

// ScreenToWorld maps DIP coordinates inside rect (relative to rect.X, rect.Y or absolute) to world coordinates.
func (s *State) ScreenToWorld(sx, sy float32, rect ui.Rect) (wx, wy float32) {
	cx := rect.X + rect.W/2
	cy := rect.Y + rect.H/2
	return (sx-cx)/s.Zoom + s.PanX, (sy-cy)/s.Zoom + s.PanY
}

// ZoomAt zooms smoothly while keeping the world point under (sx, sy) fixed.
func (s *State) ZoomAt(sx, sy float32, rect ui.Rect, factor float32) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	wx, wy := s.ScreenToWorld(sx, sy, rect)
	newZoom := s.Zoom * factor
	if newZoom < 0.18 {
		newZoom = 0.18
	}
	if newZoom > 3.5 {
		newZoom = 3.5
	}
	s.Zoom = newZoom
	cx := rect.X + rect.W/2
	cy := rect.Y + rect.H/2
	s.PanX = wx - (sx-cx)/s.Zoom
	s.PanY = wy - (sy-cy)/s.Zoom
}

// FitToView frames all visible nodes inside rect with comfortable margins.
func (s *State) FitToView(rect ui.Rect) {
	if rect.W <= 20 || rect.H <= 20 {
		return
	}
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	count := 0
	for _, id := range s.NodeOrder {
		if !s.IsNodeVisible(id) {
			continue
		}
		nv := s.Nodes[id]
		if nv.X-nv.Radius < minX {
			minX = nv.X - nv.Radius
		}
		if nv.Y-nv.Radius < minY {
			minY = nv.Y - nv.Radius
		}
		if nv.X+nv.Radius > maxX {
			maxX = nv.X + nv.Radius
		}
		if nv.Y+nv.Radius > maxY {
			maxY = nv.Y + nv.Radius
		}
		count++
	}
	if count == 0 {
		s.PanX, s.PanY, s.Zoom = 0, 0, 1
		s.NeedsFit = false
		return
	}
	s.PanX = (minX + maxX) / 2
	s.PanY = (minY + maxY) / 2
	spanX := max(maxX-minX+90, 120)
	spanY := max(maxY-minY+85, 120)
	zx := (rect.W - 44) / spanX
	zy := (rect.H - 54) / spanY
	z := zx
	if zy < z {
		z = zy
	}
	if z > 1.35 {
		z = 1.35
	}
	if z < 0.25 {
		z = 0.25
	}
	s.Zoom = z
	s.NeedsFit = false
}

// FocusNode centers the camera on a specific node ID.
func (s *State) FocusNode(id string) {
	if nv, ok := s.Nodes[id]; ok {
		s.PanX = nv.X
		s.PanY = nv.Y
		if s.Zoom < 0.85 {
			s.Zoom = 1.0
		}
	}
}

// IsNodeVisible checks tag & manual hide filters.
func (s *State) IsNodeVisible(id string) bool {
	nv, ok := s.Nodes[id]
	if !ok || s.HiddenNodes[id] {
		return false
	}
	if s.HiddenTags[nv.Vertex.PrimaryTag()] {
		return false
	}
	return true
}

// IsEdgeVisible checks edge type and endpoint visibility.
func (s *State) IsEdgeVisible(e *model.Edge) bool {
	if e == nil || s.HiddenEdgeTypes[e.Name] {
		return false
	}
	return s.IsNodeVisible(e.SrcID) && s.IsNodeVisible(e.DstID)
}

// MatchesSearch checks whether a node matches the active search filter.
func (s *State) MatchesSearch(nv *NodeVisual) bool {
	q := strings.ToLower(strings.TrimSpace(s.SearchQuery))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(nv.ID), q) {
		return true
	}
	for _, t := range nv.Vertex.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	for _, kv := range nv.Vertex.AllPropertiesFlat() {
		if strings.Contains(strings.ToLower(kv.Value), q) {
			return true
		}
	}
	return false
}

// NeighborSet returns the set of node IDs directly connected to centerID (including centerID).
func (s *State) NeighborSet(centerID string) map[string]bool {
	out := map[string]bool{centerID: true}
	for _, c := range s.Curves {
		e := c.Edge
		if !s.IsEdgeVisible(e) {
			continue
		}
		if e.SrcID == centerID {
			out[e.DstID] = true
		} else if e.DstID == centerID {
			out[e.SrcID] = true
		}
	}
	return out
}

// IsolateNeighborhood hides all nodes outside the 1-hop neighborhood of centerID.
func (s *State) IsolateNeighborhood(centerID string) {
	keep := s.NeighborSet(centerID)
	for _, id := range s.NodeOrder {
		if !keep[id] {
			s.HiddenNodes[id] = true
		}
	}
}

// ShowAllNodes clears all manually hidden nodes and tag/edge filters.
func (s *State) ShowAllNodes() {
	s.HiddenNodes = make(map[string]bool)
	s.HiddenTags = make(map[string]bool)
	s.HiddenEdgeTypes = make(map[string]bool)
}

// UnpinAll unpins every node so force layout can freely rearrange them.
func (s *State) UnpinAll() {
	for _, nv := range s.Nodes {
		nv.Pinned = false
	}
}

// TagColor returns a deterministic palette color for a tag name.
func (s *State) TagColor(tag string) ui.Color {
	if c, ok := s.tagColors[tag]; ok {
		return c
	}
	c := tagPalette[len(s.tagColors)%len(tagPalette)]
	s.tagColors[tag] = c
	return c
}

// EdgeColor returns a deterministic color for an edge type name.
func (s *State) EdgeColor(name string) ui.Color {
	if c, ok := s.edgeColors[name]; ok {
		return c
	}
	c := edgePalette[len(s.edgeColors)%len(edgePalette)]
	s.edgeColors[name] = c
	return c
}

// HitTestNode finds the topmost visible node containing world point (wx, wy).
func (s *State) HitTestNode(wx, wy float32) string {
	for i := len(s.NodeOrder) - 1; i >= 0; i-- {
		id := s.NodeOrder[i]
		if !s.IsNodeVisible(id) {
			continue
		}
		nv := s.Nodes[id]
		dx := wx - nv.X
		dy := wy - nv.Y
		r := nv.Radius + 4
		if dx*dx+dy*dy <= r*r {
			return id
		}
	}
	return ""
}

// HitTestEdge finds the closest visible edge within tol world units of (wx, wy).
func (s *State) HitTestEdge(wx, wy, tol float32) string {
	bestID := ""
	bestDist := tol
	for _, c := range s.Curves {
		e := c.Edge
		if !s.IsEdgeVisible(e) {
			continue
		}
		ns := s.Nodes[e.SrcID]
		nd := s.Nodes[e.DstID]
		if ns == nil || nd == nil {
			continue
		}
		var d float32
		if c.IsLoop {
			lx := ns.X + float32(math.Cos(float64(c.LoopAngle)))*(ns.Radius+26)
			ly := ns.Y + float32(math.Sin(float64(c.LoopAngle)))*(ns.Radius+26)
			dx, dy := wx-lx, wy-ly
			d = float32(math.Abs(math.Sqrt(float64(dx*dx+dy*dy)) - 22))
		} else {
			cpx, cpy := edgeControlPoint(ns.X, ns.Y, nd.X, nd.Y, c.Curve)
			d = distToQuadBezier(wx, wy, ns.X, ns.Y, cpx, cpy, nd.X, nd.Y)
		}
		if d < bestDist {
			bestDist = d
			bestID = e.ID
		}
	}
	return bestID
}

func edgeControlPoint(x1, y1, x2, y2, curve float32) (cpx, cpy float32) {
	mx, my := (x1+x2)/2, (y1+y2)/2
	if curve == 0 {
		return mx, my
	}
	dx, dy := x2-x1, y2-y1
	l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if l < 1 {
		return mx, my
	}
	nx, ny := -dy/l, dx/l
	return mx + nx*curve, my + ny*curve
}

func quadBezierPoint(t, x0, y0, cx, cy, x1, y1 float32) (x, y float32) {
	mt := 1 - t
	x = mt*mt*x0 + 2*mt*t*cx + t*t*x1
	y = mt*mt*y0 + 2*mt*t*cy + t*t*y1
	return x, y
}

func distToQuadBezier(px, py, x0, y0, cx, cy, x1, y1 float32) float32 {
	best := float32(math.MaxFloat32)
	const steps = 14
	prevX, prevY := x0, y0
	for i := 1; i <= steps; i++ {
		t := float32(i) / steps
		x, y := quadBezierPoint(t, x0, y0, cx, cy, x1, y1)
		d := distToSegment(px, py, prevX, prevY, x, y)
		if d < best {
			best = d
		}
		prevX, prevY = x, y
	}
	return best
}

func distToSegment(px, py, ax, ay, bx, by float32) float32 {
	vx, vy := bx-ax, by-ay
	wx, wy := px-ax, py-ay
	c1 := vx*wx + vy*wy
	if c1 <= 0 {
		return float32(math.Hypot(float64(px-ax), float64(py-ay)))
	}
	c2 := vx*vx + vy*vy
	if c2 <= c1 {
		return float32(math.Hypot(float64(px-bx), float64(py-by)))
	}
	t := c1 / c2
	projX, projY := ax+t*vx, ay+t*vy
	return float32(math.Hypot(float64(px-projX), float64(py-projY)))
}

// ExportSVG renders the current visible graph layout as a standalone vector SVG document.
func (s *State) ExportSVG(dark bool) string {
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	count := 0
	for _, id := range s.NodeOrder {
		if !s.IsNodeVisible(id) {
			continue
		}
		nv := s.Nodes[id]
		if nv.X-60 < minX {
			minX = nv.X - 60
		}
		if nv.Y-60 < minY {
			minY = nv.Y - 60
		}
		if nv.X+60 > maxX {
			maxX = nv.X + 60
		}
		if nv.Y+60 > maxY {
			maxY = nv.Y + 60
		}
		count++
	}
	if count == 0 {
		minX, minY, maxX, maxY = -200, -150, 200, 150
	}
	w := maxX - minX
	h := maxY - minY
	bg := "#ffffff"
	fg := "#18181b"
	edgeStroke := "#94a3b8"
	if dark {
		bg = "#0f1117"
		fg = "#f4f4f5"
		edgeStroke = "#64748b"
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%.1f %.1f %.1f %.1f" width="%.0f" height="%.0f">`+"\n", minX, minY, w, h, w, h)
	fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`+"\n", minX, minY, w, h, bg)
	fmt.Fprintf(&b, `<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M 0 1 L 10 5 L 0 9 z" fill="%s"/></marker></defs>`+"\n", edgeStroke)

	// Edges
	for _, c := range s.Curves {
		e := c.Edge
		if !s.IsEdgeVisible(e) {
			continue
		}
		ns, nd := s.Nodes[e.SrcID], s.Nodes[e.DstID]
		if c.IsLoop {
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="22" fill="none" stroke="%s" stroke-width="1.5"/>`+"\n",
				ns.X, ns.Y-ns.Radius-18, edgeStroke)
			continue
		}
		cpx, cpy := edgeControlPoint(ns.X, ns.Y, nd.X, nd.Y, c.Curve)
		// Trim endpoint to stop at target node radius
		tx, ty := nd.X-cpx, nd.Y-cpy
		tl := float32(math.Hypot(float64(tx), float64(ty)))
		ex, ey := nd.X, nd.Y
		if tl > 1 {
			ex = nd.X - (tx/tl)*(nd.Radius+3)
			ey = nd.Y - (ty/tl)*(nd.Radius+3)
		}
		fmt.Fprintf(&b, `<path d="M %.1f %.1f Q %.1f %.1f %.1f %.1f" fill="none" stroke="%s" stroke-width="1.5" marker-end="url(#arrow)"/>`+"\n",
			ns.X, ns.Y, cpx, cpy, ex, ey, edgeStroke)
		if s.ShowEdgeLabels {
			lx, ly := quadBezierPoint(0.5, ns.X, ns.Y, cpx, cpy, nd.X, nd.Y)
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s" font-family="system-ui,sans-serif" font-size="10" text-anchor="middle">%s</text>`+"\n",
				lx, ly-4, edgeStroke, html.EscapeString(e.DisplayLabel(false)))
		}
	}

	// Nodes
	for _, id := range s.NodeOrder {
		if !s.IsNodeVisible(id) {
			continue
		}
		nv := s.Nodes[id]
		col := colorHex(s.TagColor(nv.Vertex.PrimaryTag()))
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s" stroke="%s" stroke-width="2"/>`+"\n",
			nv.X, nv.Y, nv.Radius, col, bg)
		label := nv.Vertex.DisplayLabel(s.LabelProperty)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s" font-family="system-ui,sans-serif" font-size="11" font-weight="600" text-anchor="middle">%s</text>`+"\n",
			nv.X, nv.Y+nv.Radius+14, fg, html.EscapeString(label))
	}

	b.WriteString("</svg>\n")
	return b.String()
}

// ExportJSON returns the visible vertices and edges as pretty JSON.
func (s *State) ExportJSON() string {
	var verts []*model.Vertex
	var edges []*model.Edge
	for _, id := range s.NodeOrder {
		if s.IsNodeVisible(id) {
			verts = append(verts, s.Nodes[id].Vertex)
		}
	}
	for _, c := range s.Curves {
		if s.IsEdgeVisible(c.Edge) {
			edges = append(edges, c.Edge)
		}
	}
	b, _ := json.MarshalIndent(map[string]any{
		"vertex_count": len(verts),
		"edge_count":   len(edges),
		"vertices":     verts,
		"edges":        edges,
	}, "", "  ")
	return string(b)
}

func colorHex(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}
