package ngql

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui/editor"

	"nebula_client/internal/model"
)

type snippetDef struct {
	label  string
	detail string
	doc    string
	body   string
}

func buildSnippets(schema *model.SpaceSchema) []snippetDef {
	tag := "tag_name"
	edge := "edge_type"
	if schema != nil {
		if len(schema.Tags) > 0 {
			tag = schema.Tags[0].Name
		}
		if len(schema.Edges) > 0 {
			edge = schema.Edges[0].Name
		}
	}
	return []snippetDef{
		{
			label:  "match-path",
			detail: "MATCH path pattern",
			doc:    "Match vertices and edges into a visualizable path p",
			body:   fmt.Sprintf("MATCH p=(v:${1:%s})-[e:${2:%s}]->(n)\nRETURN p\nLIMIT ${3:25};", tag, edge),
		},
		{
			label:  "go-graph",
			detail: "GO traversal with graph objects",
			doc:    "Traverse outgoing edges and yield vertices + edge for Graph View visualization",
			body:   fmt.Sprintf("GO ${1:1} TO ${2:2} STEPS FROM \"${3:vid}\" OVER ${4:%s}\nYIELD $^ AS src, edge AS e, $$ AS dst;", edge),
		},
		{
			label:  "get-subgraph",
			detail: "GET SUBGRAPH N STEPS",
			doc:    "Fetch an N-hop neighborhood subgraph around a vertex",
			body:   fmt.Sprintf("GET SUBGRAPH WITH PROP ${1:2} STEPS FROM \"${2:vid}\" BOTH ${3:%s}\nYIELD VERTICES AS nodes, EDGES AS rels;", edge),
		},
		{
			label:  "shortest-path",
			detail: "FIND SHORTEST PATH",
			doc:    "Compute the shortest path between two vertex IDs",
			body:   fmt.Sprintf("FIND SHORTEST PATH FROM \"${1:src_vid}\" TO \"${2:dst_vid}\" OVER ${3:%s} BIDIRECT\nYIELD path AS p;", edge),
		},
		{
			label:  "fetch-prop",
			detail: "FETCH PROP ON tag",
			doc:    "Fetch properties of specific vertices by VID",
			body:   fmt.Sprintf("FETCH PROP ON ${1:%s} \"${2:vid}\"\nYIELD vertex AS v;", tag),
		},
	}
}

var keywordDocs = map[string]string{
	"MATCH":    "OpenCypher-compatible pattern matching clause.\nExample: MATCH p=(v:player)-[e:follow]->(n) RETURN p LIMIT 25;",
	"GO":       "Native nGQL multi-hop traversal starting from explicit VIDs.\nExample: GO 2 STEPS FROM \"player100\" OVER follow YIELD dst(edge);",
	"SUBGRAPH": "Returns a multi-hop subgraph (vertices & edges) around seed VIDs.\nExample: GET SUBGRAPH 2 STEPS FROM \"player100\" YIELD VERTICES AS nodes, EDGES AS rels;",
	"FETCH":    "Retrieves properties of specified vertices or edges directly by VID.\nExample: FETCH PROP ON player \"player100\" YIELD vertex AS v;",
	"LOOKUP":   "Scans vertices or edges using a secondary index.\nExample: LOOKUP ON player WHERE player.age > 35 YIELD id(vertex), player.name;",
	"SHORTEST": "Computes shortest paths between source and destination VIDs.\nExample: FIND SHORTEST PATH FROM \"a\" TO \"b\" OVER * BIDIRECT YIELD path AS p;",
	"YIELD":    "Defines the output columns returned by native nGQL statements (GO, FETCH, LOOKUP, GET SUBGRAPH).",
	"RETURN":   "Defines the output columns returned by a MATCH statement.",
	"EXPLAIN":  "Displays the physical execution plan without running the query.",
	"PROFILE":  "Executes the query and annotates the physical execution plan with runtime row counts and latency.",
	"USE":      "Switches the active graph space for the current session.",
	"SHOW":     "Inspects cluster or space metadata: SHOW SPACES / TAGS / EDGES / STATS / TAG INDEXES / HOSTS.",
	"DESCRIBE": "Shows schema properties for a SPACE, TAG, or EDGE.",
}

var funcDocs = map[string]string{
	"id":            "id(vertex) -> VID of a vertex",
	"tags":          "tags(vertex) -> list of tag names attached to a vertex",
	"labels":        "labels(vertex) -> list of tag names (OpenCypher alias for tags())",
	"properties":    "properties(vertex|edge) -> map of all properties",
	"type":          "type(edge) -> edge type name",
	"src":           "src(edge) -> source vertex ID of an edge",
	"dst":           "dst(edge) -> destination vertex ID of an edge",
	"rank":          "rank(edge) -> int64 ranking value of an edge",
	"nodes":         "nodes(path) -> list of vertices along a path",
	"relationships": "relationships(path) -> list of edges along a path",
	"length":        "length(path) -> number of edges in a path",
	"count":         "count(expr) -> aggregate count of non-null values",
	"collect":       "collect(expr) -> aggregates values into a list",
}

// Completions builds schema-aware autocomplete items for the nGQL code editor.
func Completions(schema *model.SpaceSchema) []editor.Completion {
	var items []editor.Completion

	// 1. Live Schema items first (highest value for graph queries!)
	if schema != nil {
		for _, tag := range schema.Tags {
			var fields []string
			for _, p := range tag.Properties {
				fields = append(fields, fmt.Sprintf("%s: %s", p.Field, p.Type))
			}
			doc := fmt.Sprintf("Tag `%s` in space `%s`", tag.Name, schema.Space)
			if len(fields) > 0 {
				doc += "\nProperties: " + strings.Join(fields, ", ")
			}
			items = append(items, editor.Completion{
				Label:         tag.Name,
				Detail:        "Tag",
				Kind:          editor.CompletionType,
				Documentation: doc,
			})
			for _, p := range tag.Properties {
				items = append(items, editor.Completion{
					Label:         tag.Name + "." + p.Field,
					Detail:        p.Type,
					Kind:          editor.CompletionField,
					Documentation: fmt.Sprintf("Property `%s.%s` (%s)", tag.Name, p.Field, p.Type),
				})
			}
		}

		for _, edge := range schema.Edges {
			var fields []string
			for _, p := range edge.Properties {
				fields = append(fields, fmt.Sprintf("%s: %s", p.Field, p.Type))
			}
			doc := fmt.Sprintf("Edge Type `%s` in space `%s`", edge.Name, schema.Space)
			if len(fields) > 0 {
				doc += "\nProperties: " + strings.Join(fields, ", ")
			}
			items = append(items, editor.Completion{
				Label:         edge.Name,
				Detail:        "Edge",
				Kind:          editor.CompletionModule,
				Documentation: doc,
			})
		}

		for _, prop := range schema.AllPropertyNames() {
			items = append(items, editor.Completion{
				Label:  prop,
				Detail: "Property",
				Kind:   editor.CompletionField,
			})
		}
	}

	// 2. Query snippets (adapted to current space schema)
	for _, s := range buildSnippets(schema) {
		snip, err := editor.ParseSnippet(s.body)
		comp := editor.Completion{
			Label:         s.label,
			Detail:        s.detail,
			Kind:          editor.CompletionSnippet,
			Documentation: s.doc,
		}
		if err == nil {
			comp.Snippet = snip
		} else {
			comp.Insert = s.body
		}
		items = append(items, comp)
	}

	// 3. Built-in functions
	fnNames := make([]string, 0, len(funcDocs))
	for k := range funcDocs {
		fnNames = append(fnNames, k)
	}
	sort.Strings(fnNames)
	for _, fn := range fnNames {
		items = append(items, editor.Completion{
			Label:         fn,
			Detail:        "function",
			Kind:          editor.CompletionFunction,
			Insert:        fn + "()",
			Documentation: funcDocs[fn],
		})
	}

	// 4. nGQL Keywords
	kwList := make([]string, 0, len(keywords))
	for kw := range keywords {
		kwList = append(kwList, kw)
	}
	sort.Strings(kwList)
	for _, kw := range kwList {
		items = append(items, editor.Completion{
			Label:         kw,
			Detail:        "nGQL",
			Kind:          editor.CompletionKeyword,
			Documentation: keywordDocs[kw],
		})
	}

	return items
}

// HoverDoc returns hover documentation for a word under the pointer, checking schema first.
func HoverDoc(word string, schema *model.SpaceSchema) (string, bool) {
	if word == "" {
		return "", false
	}
	if schema != nil {
		for _, t := range schema.Tags {
			if strings.EqualFold(t.Name, word) {
				var lines []string
				lines = append(lines, fmt.Sprintf("TAG `%s` (%d vertices)", t.Name, t.Count))
				for _, p := range t.Properties {
					lines = append(lines, fmt.Sprintf("  • %s: %s", p.Field, p.Type))
				}
				return strings.Join(lines, "\n"), true
			}
		}
		for _, e := range schema.Edges {
			if strings.EqualFold(e.Name, word) {
				var lines []string
				lines = append(lines, fmt.Sprintf("EDGE `%s` (%d edges)", e.Name, e.Count))
				for _, p := range e.Properties {
					lines = append(lines, fmt.Sprintf("  • %s: %s", p.Field, p.Type))
				}
				return strings.Join(lines, "\n"), true
			}
		}
	}
	if doc, ok := keywordDocs[strings.ToUpper(word)]; ok {
		return doc, true
	}
	if doc, ok := funcDocs[strings.ToLower(word)]; ok {
		return doc, true
	}
	return "", false
}
