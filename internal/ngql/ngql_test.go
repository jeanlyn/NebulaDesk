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
