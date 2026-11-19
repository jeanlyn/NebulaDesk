package nebula

import (
	"strings"
	"testing"

	"nebula_client/internal/model"
)

func TestDemoClientQueries(t *testing.T) {
	prof := model.ConnectionProfile{
		ID:           "demo",
		Name:         "Demo",
		DefaultSpace: "basketballplayer",
		Demo:         true,
	}
	client, err := Connect(prof)
	if err != nil {
		t.Fatalf("Connect demo: %v", err)
	}
	defer client.Close()

	if !client.Connected() {
		t.Fatal("expected demo client connected")
	}

	spaces, err := client.Spaces()
	if err != nil || len(spaces) < 2 {
		t.Fatalf("Spaces() = %v, %v; want >=2 spaces", spaces, err)
	}

	schema, err := client.LoadSchema("basketballplayer")
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	if len(schema.Tags) < 3 || len(schema.Edges) < 2 {
		t.Fatalf("unexpected schema: tags=%d edges=%d", len(schema.Tags), len(schema.Edges))
	}

	// 1. MATCH path query returns graph vertices & edges
	qr, err := client.Execute(`MATCH p=(v:player)-[e:follow|serve]->(n) RETURN p LIMIT 25;`)
	if err != nil || qr.Error != "" {
		t.Fatalf("MATCH failed: err=%v qr.Error=%q", err, qr.Error)
	}
	if !qr.HasGraph() || len(qr.Graph.Vertices) < 5 || len(qr.Graph.Edges) < 5 {
		t.Fatalf("expected rich graph, got %dV %dE", len(qr.Graph.Vertices), len(qr.Graph.Edges))
	}

	// 2. GET SUBGRAPH 2 STEPS
	qr, err = client.Execute(`GET SUBGRAPH 2 STEPS FROM "player100" BOTH follow, serve YIELD VERTICES AS nodes, EDGES AS rels;`)
	if err != nil || qr.Error != "" {
		t.Fatalf("GET SUBGRAPH failed: %v / %s", err, qr.Error)
	}
	if _, ok := qr.Graph.Vertices["player100"]; !ok {
		t.Fatalf("expected seed player100 in subgraph")
	}

	// 3. FIND SHORTEST PATH
	qr, err = client.Execute(`FIND SHORTEST PATH FROM "player100" TO "player125" OVER follow, serve BIDIRECT YIELD path AS p;`)
	if err != nil || qr.Error != "" {
		t.Fatalf("FIND SHORTEST PATH failed: %v / %s", err, qr.Error)
	}
	if len(qr.Rows) != 1 || !qr.HasGraph() {
		t.Fatalf("expected 1 shortest path row with graph, got rows=%d graph=%v", len(qr.Rows), qr.HasGraph())
	}

	// 4. GO traversal
	qr, err = client.Execute(`GO 1 TO 2 STEPS FROM "player100" OVER follow YIELD dst(edge);`)
	if err != nil || qr.Error != "" || len(qr.Rows) == 0 {
		t.Fatalf("GO failed: err=%v qr.Error=%q rows=%d", err, qr.Error, len(qr.Rows))
	}

	// 5. EXPLAIN plan
	qr, err = client.Execute(`EXPLAIN MATCH p=(v:player)-[e:follow]->(m:player) WHERE v.player.age > 35 RETURN p LIMIT 10;`)
	if err != nil || qr.Error != "" || !qr.HasPlan() {
		t.Fatalf("EXPLAIN failed: err=%v qr.Error=%q plan=%d", err, qr.Error, len(qr.Plan))
	}

	// 6. USE space switch
	qr, err = client.Execute(`USE tech_ecosystem;`)
	if err != nil || qr.Error != "" || client.CurrentSpace() != "tech_ecosystem" {
		t.Fatalf("USE failed: err=%v qr.Error=%q space=%q", err, qr.Error, client.CurrentSpace())
	}
}

func TestBuildExpandQueryAndHostParsing(t *testing.T) {
	qOut := BuildExpandQuery("player100", false, "OUT", 1, 50)
	if !strings.Contains(qOut, `id(v) == "player100"`) || !strings.Contains(qOut, `]->`) || !strings.Contains(qOut, `LIMIT 50`) {
		t.Errorf("unexpected OUT query: %s", qOut)
	}
	qInt := BuildExpandQuery("42", true, "IN", 2, 30)
	if !strings.Contains(qInt, `id(v) == 42`) || !strings.Contains(qInt, `<-[e*1..2]-`) || !strings.Contains(qInt, `LIMIT 30`) {
		t.Errorf("unexpected IN int query: %s", qInt)
	}
	qBoth := BuildExpandQuery(`a"b`, false, "BOTH", 1, 50)
	if !strings.Contains(qBoth, `MATCH p=(v)-[e]-(n) WHERE id(v) == "a\"b" RETURN p LIMIT 50;`) {
		t.Errorf("unexpected BOTH bounded query: %s", qBoth)
	}

	hosts := parseHostList("10.0.0.1:9669, 10.0.0.2", 9670)
	if len(hosts) != 2 || hosts[0].Host != "10.0.0.1" || hosts[0].Port != 9669 || hosts[1].Host != "10.0.0.2" || hosts[1].Port != 9670 {
		t.Errorf("unexpected parsed hosts: %+v", hosts)
	}
}

func TestGraphDataMerge(t *testing.T) {
	g1 := model.NewGraphData()
	g1.AddVertex(&model.Vertex{ID: "v1", Tags: []string{"player"}, Properties: map[string]map[string]string{"player": {"name": "Tim"}}})
	g1.AddEdge(&model.Edge{SrcID: "v1", DstID: "v2", Name: "follow", Rank: 0})

	g2 := model.NewGraphData()
	g2.AddVertex(&model.Vertex{ID: "v1", Tags: []string{"bachelor"}, Properties: map[string]map[string]string{"bachelor": {"speciality": "Psych"}}})
	g2.AddVertex(&model.Vertex{ID: "v3", Tags: []string{"team"}})
	g2.AddEdge(&model.Edge{SrcID: "v1", DstID: "v3", Name: "serve", Rank: 0})

	addedV, addedE := g1.Merge(g2)
	if addedV != 1 || addedE != 1 {
		t.Fatalf("Merge added (%dV, %dE), want (1V, 1E)", addedV, addedE)
	}
	if len(g1.Vertices["v1"].Tags) != 2 {
		t.Errorf("expected merged multi-tag vertex v1, got tags=%v", g1.Vertices["v1"].Tags)
	}
}
