package ngql

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"

	"nebula_client/internal/model"
)

func TestNGQLTokenizer(t *testing.T) {
	lang := Language()
	if lang == nil || lang.Tokenizer == nil {
		t.Fatal("Language() returned nil tokenizer")
	}

	line := `MATCH p=(v:` + "`player`" + `)-[e:follow]->(n) WHERE id(v) == "player100" AND $var > 42 // trailing comment`
	toks, state := lang.Tokenizer.Tokenize(line, 0, nil)
	if state != 0 {
		t.Fatalf("expected state 0, got %d", state)
	}

	seen := map[editor.TokenKind]bool{}
	for _, tk := range toks {
		seen[tk.Kind] = true
	}
	for _, want := range []editor.TokenKind{
		editor.TokenKeyword,
		editor.TokenProperty,
		editor.TokenOperator,
		editor.TokenFunction,
		editor.TokenString,
		editor.TokenVariable,
		editor.TokenNumber,
		editor.TokenComment,
	} {
		if !seen[want] {
			t.Errorf("expected token kind %v to be emitted for nGQL line", want)
		}
	}

	// Multi-line block comment state carryover
	_, st1 := lang.Tokenizer.Tokenize("/* start comment", 0, nil)
	if st1 != stateBlockComment {
		t.Fatalf("expected block comment state %d, got %d", stateBlockComment, st1)
	}
	toks2, st2 := lang.Tokenizer.Tokenize("still comment */ MATCH (v) RETURN v", st1, nil)
	if st2 != 0 {
		t.Fatalf("expected state 0 after closing block comment, got %d", st2)
	}
	if len(toks2) < 2 || toks2[0].Kind != editor.TokenComment {
		t.Fatalf("expected leading comment token, got %+v", toks2)
	}
}

func TestSchemaCompletionsAndHover(t *testing.T) {
	sc := &model.SpaceSchema{
		Space: "basketballplayer",
		Tags: []model.TagSchema{
			{Name: "player", Count: 20, Properties: []model.PropertySchema{{Field: "name", Type: "string"}, {Field: "age", Type: "int64"}}},
		},
		Edges: []model.EdgeSchema{
			{Name: "follow", Count: 28, Properties: []model.PropertySchema{{Field: "degree", Type: "int64"}}},
		},
	}

	items := Completions(sc)
	foundTag, foundEdge, foundProp, foundSnippet := false, false, false, false
	for _, it := range items {
		switch {
		case it.Label == "player" && it.Kind == editor.CompletionType:
			foundTag = true
		case it.Label == "follow" && it.Kind == editor.CompletionModule:
			foundEdge = true
		case it.Label == "player.age" && it.Kind == editor.CompletionField:
			foundProp = true
		case it.Kind == editor.CompletionSnippet && it.Snippet != nil:
			foundSnippet = true
		}
	}
	if !foundTag || !foundEdge || !foundProp || !foundSnippet {
		t.Fatalf("missing completion categories: tag=%v edge=%v prop=%v snippet=%v", foundTag, foundEdge, foundProp, foundSnippet)
	}

	doc, ok := HoverDoc("player", sc)
	if !ok || !strings.Contains(doc, "name: string") {
		t.Fatalf("unexpected HoverDoc(player): ok=%v doc=%q", ok, doc)
	}
	if _, ok := HoverDoc("MATCH", sc); !ok {
		t.Fatal("expected HoverDoc for MATCH keyword")
	}
}

func sampleTestSchema() *model.SpaceSchema {
	return &model.SpaceSchema{
		Space:   "basketballplayer",
		VIDType: "FIXED_STRING(32)",
		Tags: []model.TagSchema{
			{
				Name:  "player",
				Count: 20,
				Properties: []model.PropertySchema{
					{Field: "name", Type: "string"},
					{Field: "age", Type: "int64"},
				},
			},
			{
				Name:  "team",
				Count: 10,
				Properties: []model.PropertySchema{
					{Field: "name", Type: "string"},
				},
			},
		},
		Edges: []model.EdgeSchema{
			{
				Name:  "follow",
				Count: 28,
				Properties: []model.PropertySchema{
					{Field: "degree", Type: "int64"},
				},
			},
			{
				Name:  "serve",
				Count: 24,
				Properties: []model.PropertySchema{
					{Field: "start_year", Type: "int64"},
					{Field: "end_year", Type: "int64"},
				},
			},
		},
		Indexes: []model.IndexSchema{
			{Name: "player_age_idx", Kind: "TAG", Target: "player", Fields: []string{"age"}},
			{Name: "follow_degree_idx", Kind: "EDGE", Target: "follow", Fields: []string{"degree"}},
		},
	}
}

func hasCompletion(items []editor.Completion, label string) bool {
	for _, it := range items {
		if it.Label == label {
			return true
		}
	}
	return false
}

func completionLabels(items []editor.Completion) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func TestContextAwareCompletions(t *testing.T) {
	sc := sampleTestSchema()
	spaces := []string{"basketballplayer", "tech_ecosystem"}
	vids := []string{"player100", "player101", "team200"}

	t.Run("VertexPatternAfterColon", func(t *testing.T) {
		q := "MATCH (v:"
		items := Complete(CompletionContext{Text: q, Offset: len(q), Schema: sc, Spaces: spaces})
		if !hasCompletion(items, "player") || !hasCompletion(items, "team") {
			t.Fatalf("expected tags player & team after '(v:', got %v", completionLabels(items))
		}
		if hasCompletion(items, "follow") || hasCompletion(items, "MATCH") {
			t.Fatalf("did not expect edges or keywords after '(v:', got %v", completionLabels(items))
		}
	})

	t.Run("EdgePatternAfterColonAndUnionPipe", func(t *testing.T) {
		q := "MATCH (v:player)-[e:"
		items := Complete(CompletionContext{Text: q, Offset: len(q), Schema: sc})
		if !hasCompletion(items, "follow") || !hasCompletion(items, "serve") {
			t.Fatalf("expected edges follow & serve after '-[e:', got %v", completionLabels(items))
		}
		if hasCompletion(items, "player") {
			t.Fatalf("did not expect tags inside edge pattern, got %v", completionLabels(items))
		}

		q2 := "MATCH (v:player)-[e:follow|s"
		items2 := Complete(CompletionContext{Text: q2, Offset: len(q2), Prefix: "s", Schema: sc})
		if !hasCompletion(items2, "serve") || hasCompletion(items2, "follow") {
			t.Fatalf("expected remaining edge 'serve' after 'follow|', got %v", completionLabels(items2))
		}
	})

	t.Run("InlinePropertyMapInPattern", func(t *testing.T) {
		q := "MATCH (v:player {"
		items := Complete(CompletionContext{Text: q, Offset: len(q), Schema: sc, Explicit: true})
		if !hasCompletion(items, "name") || !hasCompletion(items, "age") {
			t.Fatalf("expected player props inside '{', got %v", completionLabels(items))
		}
		if hasCompletion(items, "degree") {
			t.Fatalf("did not expect edge prop inside vertex map, got %v", completionLabels(items))
		}
	})

	t.Run("VertexVarDotAndTwoLevelDot", func(t *testing.T) {
		q := "MATCH (v:player)-[e:follow]->(n:team) WHERE v."
		items := Complete(CompletionContext{Text: q, Offset: len(q), Schema: sc})
		if !hasCompletion(items, "player.name") || !hasCompletion(items, "player.age") {
			t.Fatalf("expected v. to suggest player.name & player.age, got %v", completionLabels(items))
		}
		if hasCompletion(items, "team.name") || hasCompletion(items, "degree") {
			t.Fatalf("did not expect unrelated team/edge props on v., got %v", completionLabels(items))
		}

		q2 := "MATCH (v:player) WHERE v.player."
		items2 := Complete(CompletionContext{Text: q2, Offset: len(q2), Schema: sc})
		if !hasCompletion(items2, "name") || !hasCompletion(items2, "age") || len(items2) != 2 {
			t.Fatalf("expected exact player properties after v.player., got %v", completionLabels(items2))
		}
	})

	t.Run("EdgeVarDot", func(t *testing.T) {
		q := "MATCH (v:player)-[e:follow]->(n) WHERE e."
		items := Complete(CompletionContext{Text: q, Offset: len(q), Schema: sc})
		if !hasCompletion(items, "degree") || len(items) != 1 {
			t.Fatalf("expected only 'degree' after e., got %v", completionLabels(items))
		}
	})

	t.Run("NativeNGQLVariablesDot", func(t *testing.T) {
		q1 := `GO 1 STEPS FROM "player100" OVER follow YIELD $^.`
		items1 := Complete(CompletionContext{Text: q1, Offset: len(q1), Schema: sc})
		if !hasCompletion(items1, "player.name") || !hasCompletion(items1, "team.name") {
			t.Fatalf("expected tag.prop after $^., got %v", completionLabels(items1))
		}

		q2 := `GO 1 STEPS FROM "player100" OVER follow YIELD $^.player.`
		items2 := Complete(CompletionContext{Text: q2, Offset: len(q2), Schema: sc})
		if !hasCompletion(items2, "age") || len(items2) != 2 {
			t.Fatalf("expected player fields after $^.player., got %v", completionLabels(items2))
		}

		q3 := `GO 1 STEPS FROM "player100" OVER follow YIELD dst(edge) AS friend_id, follow.degree AS deg | GO 1 STEPS FROM $-.`
		items3 := Complete(CompletionContext{Text: q3, Offset: len(q3), Schema: sc})
		if !hasCompletion(items3, "friend_id") || !hasCompletion(items3, "deg") {
			t.Fatalf("expected piped aliases after $-., got %v", completionLabels(items3))
		}

		q4 := `GO 1 STEPS FROM "player100" OVER follow YIELD follow.`
		items4 := Complete(CompletionContext{Text: q4, Offset: len(q4), Schema: sc})
		if !hasCompletion(items4, "degree") || !hasCompletion(items4, "_dst") {
			t.Fatalf("expected follow props and edge meta props after follow., got %v", completionLabels(items4))
		}
	})

	t.Run("UseShowDescribeLookupOver", func(t *testing.T) {
		qUse := "USE "
		itemsUse := Complete(CompletionContext{Text: qUse, Offset: len(qUse), Schema: sc, Spaces: spaces, Explicit: true})
		if !hasCompletion(itemsUse, "basketballplayer") || !hasCompletion(itemsUse, "tech_ecosystem") {
			t.Fatalf("expected spaces after USE, got %v", completionLabels(itemsUse))
		}

		qShow := "SHOW "
		itemsShow := Complete(CompletionContext{Text: qShow, Offset: len(qShow), Schema: sc, Explicit: true})
		if !hasCompletion(itemsShow, "TAGS") || !hasCompletion(itemsShow, "TAG INDEXES") || !hasCompletion(itemsShow, "STATS") {
			t.Fatalf("expected SHOW subcommands, got %v", completionLabels(itemsShow))
		}

		qDescTag := "DESCRIBE TAG "
		itemsDescTag := Complete(CompletionContext{Text: qDescTag, Offset: len(qDescTag), Schema: sc, Explicit: true})
		if !hasCompletion(itemsDescTag, "player") || hasCompletion(itemsDescTag, "follow") {
			t.Fatalf("expected only tags after DESCRIBE TAG, got %v", completionLabels(itemsDescTag))
		}

		qLookup := "LOOKUP ON "
		itemsLookup := Complete(CompletionContext{Text: qLookup, Offset: len(qLookup), Schema: sc, Explicit: true})
		if len(itemsLookup) == 0 || itemsLookup[0].Label != "player" || !strings.Contains(itemsLookup[0].Detail, "indexed") {
			t.Fatalf("expected indexed tag 'player' first after LOOKUP ON, got %+v", itemsLookup)
		}

		qOver := `GO 2 STEPS FROM "player100" OVER `
		itemsOver := Complete(CompletionContext{Text: qOver, Offset: len(qOver), Schema: sc, Explicit: true})
		if !hasCompletion(itemsOver, "follow") || !hasCompletion(itemsOver, "*") || hasCompletion(itemsOver, "player") {
			t.Fatalf("expected edges + '*' after OVER, got %v", completionLabels(itemsOver))
		}
	})

	t.Run("ReturnVariablesAndShortcuts", func(t *testing.T) {
		q := "MATCH p=(v:player)-[e:follow]->(n:team) RETURN "
		items := Complete(CompletionContext{Text: q, Offset: len(q), Schema: sc, Explicit: true})
		if len(items) < 4 || items[0].Label != "p" || items[1].Label != "v" || items[2].Label != "e" || items[3].Label != "n" {
			t.Fatalf("expected bound variables p,v,e,n first in RETURN, got %v", completionLabels(items[:min(8, len(items))]))
		}
		if !hasCompletion(items, "v.player.age") || !hasCompletion(items, "e.degree") {
			t.Fatalf("expected variable property shortcuts in RETURN, got %v", completionLabels(items))
		}
	})

	t.Run("DDLDataTypes", func(t *testing.T) {
		q := "CREATE TAG person (name "
		items := Complete(CompletionContext{Text: q, Offset: len(q), Schema: sc, Explicit: true})
		if !hasCompletion(items, "string") || !hasCompletion(items, "int64") || !hasCompletion(items, "fixed_string") {
			t.Fatalf("expected NebulaGraph data types in CREATE TAG, got %v", completionLabels(items))
		}
	})

	t.Run("SuppressionInCommentsStringsAndGt", func(t *testing.T) {
		qComment := "// MATCH (v:player) WHERE v."
		if items := Complete(CompletionContext{Text: qComment, Offset: len(qComment), Schema: sc}); len(items) != 0 {
			t.Fatalf("expected no completions in comment, got %v", completionLabels(items))
		}

		qStr := `MATCH (v:player) WHERE v.player.name == "Tim`
		if items := Complete(CompletionContext{Text: qStr, Offset: len(qStr), Prefix: "Tim", Schema: sc, SampleVIDs: vids}); len(items) != 0 {
			t.Fatalf("expected no completions in ordinary string literal, got %v", completionLabels(items))
		}

		qVIDStr := `GO 1 STEPS FROM "pl`
		itemsVID := Complete(CompletionContext{Text: qVIDStr, Offset: len(qVIDStr), Prefix: "pl", Schema: sc, SampleVIDs: vids})
		if !hasCompletion(itemsVID, "player100") {
			t.Fatalf("expected canvas VID suggestion inside FROM string, got %v", completionLabels(itemsVID))
		}

		qGt := "MATCH (v:player) WHERE v.player.age >"
		if items := Complete(CompletionContext{Text: qGt, Offset: len(qGt), Schema: sc, Explicit: false}); len(items) != 0 {
			t.Fatalf("expected no auto-popup right after '>', got %v", completionLabels(items))
		}

		qNewVar := "MATCH (v"
		if items := Complete(CompletionContext{Text: qNewVar, Offset: len(qNewVar), Prefix: "v", Schema: sc, Explicit: false}); len(items) != 0 {
			t.Fatalf("expected no noisy auto-popup while typing new variable name '(v', got %v", completionLabels(items))
		}
	})

	t.Run("IntVIDSnippets", func(t *testing.T) {
		intSchema := sampleTestSchema()
		intSchema.VIDType = "INT64"
		items := Complete(CompletionContext{Text: "", Offset: 0, Schema: intSchema, Explicit: true})
		foundGo := false
		for _, it := range items {
			if it.Label == "go-graph" && it.Snippet != nil {
				foundGo = true
				if strings.Contains(it.Snippet.Text(), `"100"`) {
					t.Fatalf("expected unquoted INT64 VID in go-graph snippet, got %q", it.Snippet.Text())
				}
			}
		}
		if !foundGo {
			t.Fatal("expected go-graph snippet at statement start")
		}
	})
}

func TestHoverAtContext(t *testing.T) {
	sc := sampleTestSchema()
	spaces := []string{"basketballplayer", "tech_ecosystem"}

	q := "MATCH (v:player)-[e:follow]->(n) WHERE v.player.age > 30 AND e.degree > 90 RETURN v;"

	// Hover on `age` in `v.player.age`
	ageOff := strings.Index(q, "age") + 1
	doc, _, _, ok := HoverAt(q, ageOff, sc, spaces)
	if !ok || !strings.Contains(doc, "player.age") || !strings.Contains(doc, "player_age_idx") {
		t.Fatalf("expected indexed property hover for v.player.age, got ok=%v doc=%q", ok, doc)
	}

	// Hover on `degree` in `e.degree`
	degOff := strings.Index(q, "degree") + 2
	doc, _, _, ok = HoverAt(q, degOff, sc, spaces)
	if !ok || !strings.Contains(doc, "follow.degree") || !strings.Contains(doc, "follow_degree_idx") {
		t.Fatalf("expected indexed edge property hover for e.degree, got ok=%v doc=%q", ok, doc)
	}

	// Hover on bound variable `v` in `RETURN v`
	retVOff := strings.LastIndex(q, "v")
	doc, _, _, ok = HoverAt(q, retVOff, sc, spaces)
	if !ok || !strings.Contains(doc, "Vertex variable") || !strings.Contains(doc, "player") {
		t.Fatalf("expected bound vertex variable hover for v, got ok=%v doc=%q", ok, doc)
	}
}
