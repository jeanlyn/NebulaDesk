package graphview

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"nebula_client/internal/model"
)

func sampleGraph() *model.GraphData {
	g := model.NewGraphData()
	g.AddVertex(&model.Vertex{ID: "p1", Tags: []string{"player"}, Properties: map[string]map[string]string{"player": {"name": "Tim Duncan"}}})
	g.AddVertex(&model.Vertex{ID: "p2", Tags: []string{"player"}, Properties: map[string]map[string]string{"player": {"name": "Tony Parker"}}})
	g.AddVertex(&model.Vertex{ID: "t1", Tags: []string{"team"}, Properties: map[string]map[string]string{"team": {"name": "Spurs"}}})
	// Bidirectional pair + multi-rank parallel edges + self-loop
	g.AddEdge(&model.Edge{SrcID: "p1", DstID: "p2", Name: "follow", Rank: 0})
	g.AddEdge(&model.Edge{SrcID: "p2", DstID: "p1", Name: "follow", Rank: 0})
	g.AddEdge(&model.Edge{SrcID: "p1", DstID: "t1", Name: "serve", Rank: 0})
	g.AddEdge(&model.Edge{SrcID: "p1", DstID: "t1", Name: "serve", Rank: 1})
	g.AddEdge(&model.Edge{SrcID: "p1", DstID: "p1", Name: "self_ref", Rank: 0})
	return g
}

func TestLayoutsCurvesAndHitTesting(t *testing.T) {
	st := NewState()
	st.SetGraph(sampleGraph())

	if len(st.Nodes) != 3 || len(st.Curves) != 5 {
		t.Fatalf("got %d nodes and %d curves, want 3 and 5", len(st.Nodes), len(st.Curves))
	}

	// Verify parallel edges get opposite non-zero Bézier curves and self-loop is flagged
	curvedCount, loopCount := 0, 0
	for _, c := range st.Curves {
		if c.IsLoop {
			loopCount++
		} else if c.Curve != 0 {
			curvedCount++
		}
	}
	if loopCount != 1 || curvedCount != 4 {
		t.Errorf("expected 1 loop and 4 curved parallel edges, got loop=%d curved=%d", loopCount, curvedCount)
	}

	// Run force simulation to convergence
	for i := 0; i < 400 && st.Simulating; i++ {
		st.StepForce(1.0)
	}
	for _, nv := range st.Nodes {
		if math.IsNaN(float64(nv.X)) || math.IsNaN(float64(nv.Y)) {
			t.Fatalf("NaN node position for %s", nv.ID)
		}
	}

	// Test every layout mode
	for _, mode := range []LayoutMode{LayoutCircular, LayoutHierarchical, LayoutGrid, LayoutForce} {
		st.ApplyLayout(mode)
		if len(st.NodeOrder) != 3 {
			t.Fatalf("layout %v corrupted node count", mode)
		}
	}

	// Test HitTestNode at exact node center
	p1 := st.Nodes["p1"]
	if hit := st.HitTestNode(p1.X, p1.Y); hit != "p1" {
		t.Errorf("HitTestNode(p1) = %q, want p1", hit)
	}

	// Test FitToView & ZoomAt
	rect := ui.Rect{X: 0, Y: 0, W: 800, H: 600}
	st.FitToView(rect)
	if st.Zoom <= 0 || st.NeedsFit {
		t.Errorf("FitToView produced invalid state: zoom=%v needsFit=%v", st.Zoom, st.NeedsFit)
	}
	beforeZoom := st.Zoom
	st.ZoomAt(400, 300, rect, 1.5)
	if st.Zoom <= beforeZoom {
		t.Errorf("ZoomAt did not zoom in: before=%v after=%v", beforeZoom, st.Zoom)
	}

	// Test ExportSVG & ExportJSON
	svg := st.ExportSVG(true)
	if !strings.HasPrefix(svg, "<svg ") || !strings.Contains(svg, "Tim Duncan") || !strings.Contains(svg, "</svg>") {
		t.Errorf("ExportSVG output malformed: %s", svg[:min(len(svg), 200)])
	}
	js := st.ExportJSON()
	if !strings.Contains(js, `"vertex_count": 3`) || !strings.Contains(js, `"edge_count": 5`) {
		t.Errorf("ExportJSON missing expected counts: %s", js)
	}
}

func TestMergeGraphPreservesPinnedNode(t *testing.T) {
	st := NewState()
	st.SetGraph(sampleGraph())
	st.Nodes["p1"].X = 123
	st.Nodes["p1"].Y = 456
	st.Nodes["p1"].Pinned = true

	extra := model.NewGraphData()
	extra.AddVertex(&model.Vertex{ID: "p3", Tags: []string{"player"}})
	extra.AddEdge(&model.Edge{SrcID: "p1", DstID: "p3", Name: "follow"})

	av, ae := st.MergeGraph(extra, "p1")
	if av != 1 || ae != 1 {
		t.Fatalf("MergeGraph added (%d,%d), want (1,1)", av, ae)
	}
	if st.Nodes["p1"].X != 123 || st.Nodes["p1"].Y != 456 {
		t.Errorf("pinned anchor node moved during merge")
	}
}

func TestLargeGraphSpatialHashPerformance(t *testing.T) {
	g := model.NewGraphData()
	const nNodes = 800
	g.AddVertex(&model.Vertex{ID: "hub0", Tags: []string{"task"}})
	for i := 1; i < nNodes; i++ {
		id := fmt.Sprintf("task_%d", i)
		g.AddVertex(&model.Vertex{ID: id, Tags: []string{"task"}})
		// Super-node star + chain edges simulating a large task DAG
		g.AddEdge(&model.Edge{SrcID: "hub0", DstID: id, Name: "task_link"})
		if i > 1 {
			g.AddEdge(&model.Edge{SrcID: fmt.Sprintf("task_%d", i-1), DstID: id, Name: "task_link"})
		}
	}

	st := NewState()
	start := time.Now()
	st.SetGraph(g)
	setupElapsed := time.Since(start)
	if setupElapsed > 150*time.Millisecond {
		t.Errorf("SetGraph(800 nodes / 1598 edges) took %v, want < 150ms", setupElapsed)
	}

	stepStart := time.Now()
	st.StepForce(1.0)
	stepElapsed := time.Since(stepStart)
	if stepElapsed > 25*time.Millisecond {
		t.Errorf("single StepForce(800 nodes) took %v, want < 25ms", stepElapsed)
	}
	t.Logf("800 nodes / 1598 edges: SetGraph+prewarm=%v, 1x StepForce=%v", setupElapsed, stepElapsed)
}
