package ngql

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui/editor"

	"nebula_client/internal/model"
)

// CompletionContext provides full editor and schema context for intelligent nGQL autocompletion.
type CompletionContext struct {
	Text       string
	Offset     int
	Prefix     string
	Explicit   bool
	Schema     *model.SpaceSchema
	Spaces     []string
	SampleVIDs []string
}

type snippetDef struct {
	label  string
	detail string
	doc    string
	body   string
}

type funcDef struct {
	name      string
	signature string
	snippet   string
	doc       string
}

var funcCatalog = []funcDef{
	// Graph element functions
	{name: "id", signature: "id(vertex) -> VID", snippet: "id($1)$0", doc: "Returns the vertex ID (VID) of a vertex."},
	{name: "tags", signature: "tags(vertex) -> list<string>", snippet: "tags($1)$0", doc: "Returns all tag names attached to a vertex."},
	{name: "labels", signature: "labels(vertex) -> list<string>", snippet: "labels($1)$0", doc: "OpenCypher alias for tags(vertex). Returns tag names of a vertex."},
	{name: "properties", signature: "properties(vertex|edge) -> map", snippet: "properties($1)$0", doc: "Returns a map of all properties on a vertex or edge."},
	{name: "type", signature: "type(edge) -> string", snippet: "type($1)$0", doc: "Returns the edge type name of an edge."},
	{name: "src", signature: "src(edge) -> VID", snippet: "src($1)$0", doc: "Returns the source vertex ID of an edge."},
	{name: "dst", signature: "dst(edge) -> VID", snippet: "dst($1)$0", doc: "Returns the destination vertex ID of an edge."},
	{name: "rank", signature: "rank(edge) -> int64", snippet: "rank($1)$0", doc: "Returns the int64 ranking value of an edge."},
	{name: "nodes", signature: "nodes(path) -> list<vertex>", snippet: "nodes($1)$0", doc: "Returns all vertices along a path."},
	{name: "relationships", signature: "relationships(path) -> list<edge>", snippet: "relationships($1)$0", doc: "Returns all edges along a path."},
	{name: "length", signature: "length(path) -> int64", snippet: "length($1)$0", doc: "Returns the number of edges (hops) in a path."},
	{name: "startNode", signature: "startNode(edge) -> vertex", snippet: "startNode($1)$0", doc: "Returns the start vertex of an edge in MATCH."},
	{name: "endNode", signature: "endNode(edge) -> vertex", snippet: "endNode($1)$0", doc: "Returns the end vertex of an edge in MATCH."},

	// Aggregation functions
	{name: "count", signature: "count(expr) -> int64", snippet: "count($1)$0", doc: "Counts non-null values (or count(*) for all rows)."},
	{name: "collect", signature: "collect(expr) -> list", snippet: "collect($1)$0", doc: "Aggregates non-null values into a list."},
	{name: "sum", signature: "sum(expr) -> number", snippet: "sum($1)$0", doc: "Computes the sum of numeric values."},
	{name: "avg", signature: "avg(expr) -> double", snippet: "avg($1)$0", doc: "Computes the average of numeric values."},
	{name: "min", signature: "min(expr) -> value", snippet: "min($1)$0", doc: "Returns the minimum value."},
	{name: "max", signature: "max(expr) -> value", snippet: "max($1)$0", doc: "Returns the maximum value."},

	// Collection & predicate functions
	{name: "size", signature: "size(list|map|string) -> int64", snippet: "size($1)$0", doc: "Returns the length of a list, map, or string."},
	{name: "keys", signature: "keys(vertex|edge|map) -> list<string>", snippet: "keys($1)$0", doc: "Returns the property/key names of a vertex, edge, or map."},
	{name: "head", signature: "head(list) -> value", snippet: "head($1)$0", doc: "Returns the first element of a list."},
	{name: "last", signature: "last(list) -> value", snippet: "last($1)$0", doc: "Returns the last element of a list."},
	{name: "tail", signature: "tail(list) -> list", snippet: "tail($1)$0", doc: "Returns all elements of a list except the first."},
	{name: "range", signature: "range(start, end[, step]) -> list<int>", snippet: "range(${1:0}, ${2:10})$0", doc: "Generates a list of integers from start to end inclusive."},
	{name: "coalesce", signature: "coalesce(expr1, expr2, ...) -> value", snippet: "coalesce(${1:expr}, ${2:default})$0", doc: "Returns the first non-null expression."},
	{name: "exists", signature: "exists(expr) -> bool", snippet: "exists($1)$0", doc: "Checks whether a property or pattern exists."},
	{name: "isEmpty", signature: "isEmpty(list|map|string) -> bool", snippet: "isEmpty($1)$0", doc: "Returns true if a list, map, or string is empty."},

	// String functions
	{name: "toLower", signature: "toLower(str) -> string", snippet: "toLower($1)$0", doc: "Converts a string to lowercase."},
	{name: "toUpper", signature: "toUpper(str) -> string", snippet: "toUpper($1)$0", doc: "Converts a string to uppercase."},
	{name: "trim", signature: "trim(str) -> string", snippet: "trim($1)$0", doc: "Removes leading and trailing whitespace."},
	{name: "ltrim", signature: "ltrim(str) -> string", snippet: "ltrim($1)$0", doc: "Removes leading whitespace."},
	{name: "rtrim", signature: "rtrim(str) -> string", snippet: "rtrim($1)$0", doc: "Removes trailing whitespace."},
	{name: "substring", signature: "substring(str, pos[, len]) -> string", snippet: "substring(${1:str}, ${2:0}, ${3:5})$0", doc: "Extracts a substring starting at 0-based index pos."},
	{name: "replace", signature: "replace(str, from, to) -> string", snippet: "replace(${1:str}, ${2:old}, ${3:new})$0", doc: "Replaces all occurrences of a substring."},
	{name: "split", signature: "split(str, delim) -> list<string>", snippet: "split(${1:str}, ${2:\",\"})$0", doc: "Splits a string by delimiter into a list."},

	// Temporal functions
	{name: "now", signature: "now() -> int64", snippet: "now()$0", doc: "Returns the current Unix timestamp in seconds."},
	{name: "datetime", signature: "datetime([str|map]) -> datetime", snippet: "datetime($1)$0", doc: "Constructs a datetime value (or current UTC datetime when empty)."},
	{name: "date", signature: "date([str|map]) -> date", snippet: "date($1)$0", doc: "Constructs a date value."},
	{name: "time", signature: "time([str|map]) -> time", snippet: "time($1)$0", doc: "Constructs a time value."},
	{name: "duration", signature: "duration(map) -> duration", snippet: "duration($1)$0", doc: "Constructs a duration value, e.g. duration({days: 7})."},

	// Math & conversion functions
	{name: "abs", signature: "abs(num) -> number", snippet: "abs($1)$0", doc: "Returns the absolute value of a number."},
	{name: "round", signature: "round(num) -> double", snippet: "round($1)$0", doc: "Rounds a number to the nearest integer."},
	{name: "ceil", signature: "ceil(num) -> double", snippet: "ceil($1)$0", doc: "Returns the smallest integer >= num."},
	{name: "floor", signature: "floor(num) -> double", snippet: "floor($1)$0", doc: "Returns the largest integer <= num."},
	{name: "sqrt", signature: "sqrt(num) -> double", snippet: "sqrt($1)$0", doc: "Returns the square root of a number."},
	{name: "pow", signature: "pow(base, exp) -> double", snippet: "pow(${1:x}, ${2:2})$0", doc: "Returns base raised to the power exp."},
	{name: "rand", signature: "rand() -> double", snippet: "rand()$0", doc: "Returns a random float in [0.0, 1.0)."},
	{name: "toString", signature: "toString(expr) -> string", snippet: "toString($1)$0", doc: "Converts a value to a string."},
	{name: "toInteger", signature: "toInteger(expr) -> int64", snippet: "toInteger($1)$0", doc: "Converts a value to an int64."},
	{name: "toFloat", signature: "toFloat(expr) -> double", snippet: "toFloat($1)$0", doc: "Converts a value to a double."},
	{name: "toBoolean", signature: "toBoolean(expr) -> bool", snippet: "toBoolean($1)$0", doc: "Converts a value to a boolean."},
}

var funcByLower = func() map[string]funcDef {
	m := make(map[string]funcDef, len(funcCatalog))
	for _, f := range funcCatalog {
		m[strings.ToLower(f.name)] = f
	}
	return m
}()

var keywordDocs = map[string]string{
	"MATCH":        "OpenCypher-compatible pattern matching clause.\nExample: MATCH p=(v:player)-[e:follow]->(n) RETURN p LIMIT 25;",
	"OPTIONAL":     "OPTIONAL MATCH matches patterns returning NULL for missing parts instead of filtering rows.",
	"GO":           "Native nGQL multi-hop traversal starting from explicit VIDs.\nExample: GO 2 STEPS FROM \"player100\" OVER follow YIELD dst(edge);",
	"GET":          "GET SUBGRAPH fetches an N-hop neighborhood (vertices & edges) around seed VIDs.",
	"SUBGRAPH":     "Returns a multi-hop subgraph (vertices & edges) around seed VIDs.\nExample: GET SUBGRAPH 2 STEPS FROM \"player100\" YIELD VERTICES AS nodes, EDGES AS rels;",
	"FETCH":        "Retrieves properties of specified vertices or edges directly by VID.\nExample: FETCH PROP ON player \"player100\" YIELD vertex AS v;",
	"LOOKUP":       "Scans vertices or edges using a secondary index.\nExample: LOOKUP ON player WHERE player.age > 35 YIELD id(vertex), player.name;",
	"FIND":         "Finds paths between source and destination VIDs (SHORTEST / ALL / NOLOOP PATH).",
	"SHORTEST":     "Computes shortest paths between source and destination VIDs.\nExample: FIND SHORTEST PATH FROM \"a\" TO \"b\" OVER * BIDIRECT YIELD path AS p;",
	"YIELD":        "Defines the output columns returned by native nGQL statements (GO, FETCH, LOOKUP, GET SUBGRAPH).",
	"RETURN":       "Defines the output columns returned by a MATCH statement.",
	"WHERE":        "Filters matched patterns, traversed edges, or indexed rows by boolean predicate.",
	"WITH":         "Chains MATCH clauses together and transforms or aggregates intermediate variables.",
	"OVER":         "Specifies edge type(s) to traverse in GO or FIND PATH (use * for all edge types).",
	"REVERSELY":    "Traverses incoming edges in reverse direction in GO / FIND PATH.",
	"BIDIRECT":     "Traverses both outgoing and incoming edges in GO / FIND PATH.",
	"FROM":         "Specifies starting vertex ID(s) in GO, GET SUBGRAPH, or FIND PATH.",
	"TO":           "Specifies destination vertex ID(s) in FIND PATH or step upper bound in GO M TO N STEPS.",
	"LIMIT":        "Caps the maximum number of returned rows.",
	"SKIP":         "Skips the first N rows before returning results (MATCH).",
	"ORDER":        "Sorts returned rows with ORDER BY <expr> [ASC|DESC].",
	"GROUP":        "Groups rows for aggregation with GROUP BY <expr> YIELD ... .",
	"UNWIND":       "Expands a list into individual rows in MATCH.",
	"DISTINCT":     "Eliminates duplicate rows in RETURN / YIELD.",
	"EXPLAIN":      "Displays the physical execution plan without running the query.",
	"PROFILE":      "Executes the query and annotates the physical execution plan with runtime row counts and latency.",
	"USE":          "Switches the active graph space for the current session.",
	"SHOW":         "Inspects cluster or space metadata: SHOW SPACES / TAGS / EDGES / STATS / TAG INDEXES / HOSTS.",
	"DESCRIBE":     "Shows schema properties for a SPACE, TAG, EDGE, or INDEX.",
	"CREATE":       "Creates a SPACE, TAG, EDGE, TAG INDEX, or EDGE INDEX.",
	"ALTER":        "Modifies an existing TAG, EDGE, or SPACE definition.",
	"DROP":         "Removes a SPACE, TAG, EDGE, or INDEX.",
	"INSERT":       "Inserts vertices (INSERT VERTEX) or edges (INSERT EDGE).",
	"UPDATE":       "Updates properties of a vertex or edge in place.",
	"UPSERT":       "Updates a vertex or edge if it exists, or inserts it otherwise.",
	"DELETE":       "Deletes vertices (DELETE VERTEX ... WITH EDGE) or edges (DELETE EDGE).",
	"SUBMIT":       "Submits an asynchronous cluster job: SUBMIT JOB STATS / COMPACT / FLUSH / REBUILD INDEX.",
	"REBUILD":      "Rebuilds a TAG INDEX or EDGE INDEX for existing data.",
}

var dataTypeDefs = []struct {
	label   string
	snippet string
	doc     string
}{
	{label: "string", doc: "Variable-length UTF-8 string."},
	{label: "fixed_string", snippet: "fixed_string(${1:32})$0", doc: "Fixed-length byte string (required for string VIDs & indexes)."},
	{label: "int64", doc: "64-bit signed integer."},
	{label: "int32", doc: "32-bit signed integer."},
	{label: "int16", doc: "16-bit signed integer."},
	{label: "int8", doc: "8-bit signed integer."},
	{label: "double", doc: "64-bit IEEE-754 floating-point number."},
	{label: "float", doc: "32-bit IEEE-754 floating-point number."},
	{label: "bool", doc: "Boolean value (true / false)."},
	{label: "timestamp", doc: "Unix timestamp in seconds."},
	{label: "date", doc: "Calendar date (YYYY-MM-DD)."},
	{label: "time", doc: "Time of day (HH:MM:SS.ffffff)."},
	{label: "datetime", doc: "Date and time with timezone."},
	{label: "duration", doc: "Time interval duration."},
	{label: "geography", doc: "Geospatial point, linestring, or polygon."},
}

var showSubcommands = []struct {
	label string
	doc   string
}{
	{"SPACES", "List all graph spaces in the cluster"},
	{"TAGS", "List all tag types in the current space"},
	{"EDGES", "List all edge types in the current space"},
	{"TAG INDEXES", "List all tag secondary indexes in the current space"},
	{"EDGE INDEXES", "List all edge secondary indexes in the current space"},
	{"STATS", "Show vertex/edge count statistics (run SUBMIT JOB STATS first)"},
	{"HOSTS", "Show storage hosts and partition leader distribution"},
	{"PARTS", "Show partition placement across storage hosts"},
	{"JOBS", "Show background job status (STATS, COMPACT, REBUILD INDEX)"},
	{"SESSIONS", "Show active client sessions"},
	{"QUERIES", "Show currently running queries"},
	{"CREATE SPACE", "Show the DDL statement for a graph space"},
	{"CREATE TAG", "Show the DDL statement for a tag"},
	{"CREATE EDGE", "Show the DDL statement for an edge type"},
	{"CREATE TAG INDEX", "Show the DDL statement for a tag index"},
	{"CREATE EDGE INDEX", "Show the DDL statement for an edge index"},
	{"TAG INDEX STATUS", "Show rebuild status of tag indexes"},
	{"EDGE INDEX STATUS", "Show rebuild status of edge indexes"},
	{"CHARSET", "Show supported character sets"},
	{"COLLATION", "Show supported collations"},
}

func isIntVIDSchema(schema *model.SpaceSchema) bool {
	return schema != nil && strings.Contains(strings.ToUpper(schema.VIDType), "INT")
}

func sampleVIDPlaceholder(schema *model.SpaceSchema, fallback string, numFallback string) string {
	if isIntVIDSchema(schema) {
		return fmt.Sprintf("${%s}", numFallback)
	}
	return fmt.Sprintf("\"${%s}\"", fallback)
}

func buildSnippets(schema *model.SpaceSchema) []snippetDef {
	tag := "tag_name"
	tagProp := "name"
	edge := "edge_type"
	lookupTag := tag
	lookupProp := tagProp

	if schema != nil {
		if len(schema.Tags) > 0 {
			tag = schema.Tags[0].Name
			lookupTag = tag
			if len(schema.Tags[0].Properties) > 0 {
				tagProp = schema.Tags[0].Properties[0].Field
				lookupProp = tagProp
			}
		}
		if len(schema.Edges) > 0 {
			edge = schema.Edges[0].Name
		}
		// Prefer an actually indexed tag/property for lookup-on snippet!
		for _, idx := range schema.Indexes {
			if strings.EqualFold(idx.Kind, "TAG") && idx.Target != "" {
				lookupTag = idx.Target
				if len(idx.Fields) > 0 {
					f := idx.Fields[0]
					if p := strings.IndexByte(f, '('); p > 0 {
						f = f[:p]
					}
					lookupProp = strings.TrimSpace(f)
				}
				break
			}
		}
	}

	vid1 := sampleVIDPlaceholder(schema, "1:vid", "1:100")
	vid2 := sampleVIDPlaceholder(schema, "2:vid", "2:100")
	vid3 := sampleVIDPlaceholder(schema, "3:vid", "3:100")
	srcVid := sampleVIDPlaceholder(schema, "1:src_vid", "1:100")
	dstVid := sampleVIDPlaceholder(schema, "2:dst_vid", "2:101")

	return []snippetDef{
		{
			label:  "match-path",
			detail: "MATCH path pattern",
			doc:    "Match vertices and edges into a visualizable path p",
			body:   fmt.Sprintf("MATCH p=(v:${1:%s})-[e:${2:%s}]->(n)\nRETURN p\nLIMIT ${3:25};", tag, edge),
		},
		{
			label:  "match-node",
			detail: "MATCH vertex filter",
			doc:    "Match vertices of a tag filtered by property value",
			body:   fmt.Sprintf("MATCH (v:${1:%s})\nWHERE v.${1:%s}.${2:%s} == ${3:\"value\"}\nRETURN v\nLIMIT ${4:25};", tag, tag, tagProp),
		},
		{
			label:  "match-varlen",
			detail: "MATCH 1..N hop paths",
			doc:    "Match variable-length paths between 1 and N hops",
			body:   fmt.Sprintf("MATCH p=(v:${1:%s})-[e:${2:%s}*1..${3:2}]->(n)\nRETURN p\nLIMIT ${4:25};", tag, edge),
		},
		{
			label:  "go-graph",
			detail: "GO traversal with graph objects",
			doc:    "Traverse outgoing edges and yield vertices + edge for Graph View visualization",
			body:   fmt.Sprintf("GO ${1:1} TO ${2:2} STEPS FROM %s OVER ${4:%s}\nYIELD \\$^ AS src, edge AS e, \\$\\$ AS dst;", vid3, edge),
		},
		{
			label:  "go-pipe",
			detail: "GO multi-stage piped traversal",
			doc:    "Chain two GO traversals using the pipe operator | and $- input variable",
			body:   fmt.Sprintf("GO 1 STEPS FROM %s OVER ${2:%s} YIELD dst(edge) AS vid |\nGO 1 STEPS FROM \\$-.vid OVER ${3:%s} YIELD \\$^ AS src, edge AS e, \\$\\$ AS dst;", vid1, edge, edge),
		},
		{
			label:  "get-subgraph",
			detail: "GET SUBGRAPH N STEPS",
			doc:    "Fetch an N-hop neighborhood subgraph around a vertex",
			body:   fmt.Sprintf("GET SUBGRAPH WITH PROP ${1:2} STEPS FROM %s BOTH ${3:%s}\nYIELD VERTICES AS nodes, EDGES AS rels;", vid2, edge),
		},
		{
			label:  "shortest-path",
			detail: "FIND SHORTEST PATH",
			doc:    "Compute the shortest path between two vertex IDs",
			body:   fmt.Sprintf("FIND SHORTEST PATH FROM %s TO %s OVER ${3:%s} BIDIRECT\nYIELD path AS p;", srcVid, dstVid, edge),
		},
		{
			label:  "all-path",
			detail: "FIND ALL PATH",
			doc:    "Find all paths up to N steps between two vertex IDs",
			body:   fmt.Sprintf("FIND ALL PATH FROM %s TO %s OVER ${3:%s} UPTO ${4:3} STEPS\nYIELD path AS p;", srcVid, dstVid, edge),
		},
		{
			label:  "fetch-prop",
			detail: "FETCH PROP ON tag",
			doc:    "Fetch properties of specific vertices by VID",
			body:   fmt.Sprintf("FETCH PROP ON ${1:%s} %s\nYIELD vertex AS v;", tag, vid2),
		},
		{
			label:  "lookup-on",
			detail: "LOOKUP ON indexed tag",
			doc:    "Scan vertices using a secondary index",
			body:   fmt.Sprintf("LOOKUP ON ${1:%s}\nWHERE ${1:%s}.${2:%s} == ${3:\"value\"}\nYIELD vertex AS v;", lookupTag, lookupTag, lookupProp),
		},
	}
}

// Complete computes context-aware, schema-aware nGQL autocompletion items at ctx.Offset.
func Complete(ctx CompletionContext) []editor.Completion {
	// If no text context was provided, fall back to full schema+snippets+funcs+keywords.
	if ctx.Text == "" && ctx.Offset == 0 && ctx.Prefix == "" && !ctx.Explicit {
		return Completions(ctx.Schema)
	}

	ac := analyzeContext(ctx.Text, ctx.Offset, ctx.Prefix)

	// 1. Never popup completions inside comments.
	if ac.inComment {
		return nil
	}

	// 2. Inside string literals: only offer known VIDs in VID positions (FROM/TO/FETCH), never spam keywords.
	if ac.inString {
		if isVIDStringPosition(ac.pipeTokens) && len(ctx.SampleVIDs) > 0 {
			return vidCompletions(ctx.SampleVIDs, ctx.Schema, false)
		}
		return nil
	}

	// 3. Suppress unwanted automatic popup right after '>' (e.g. `age >` or `]->`).
	//    Users can still explicitly invoke completions with Ctrl+Space / Option+/.
	if !ctx.Explicit && ctx.Prefix == "" && ac.prevChar == '>' {
		return nil
	}

	// 4. Dot member access: `v.`, `v.player.`, `player.`, `follow.`, `$^.`, `$$.`, `$-.`
	if chain, ok := ac.trailingDotChain(); ok {
		return completeDotChain(chain, ac, ctx.Schema, ctx.Explicit)
	}

	// 5. Inside enclosing brackets: graph pattern labels `(v:`, `[e:`, `[e:follow|`, inline props `{...}`, or DDL types `CREATE TAG t (...)`
	if items, handled := completeInsideDelimiters(ac, ctx.Schema, ctx.Explicit); handled {
		return items
	}

	// 6. Keyword / clause-specific contextual completion based on preceding tokens.
	if items, handled := completeAfterTokens(ac, ctx); handled {
		return items
	}

	// 7. Start of statement (empty or right after ';') or start of pipe segment ('|')
	if len(ac.pipeTokens) == 0 {
		if ac.hasPipe {
			return completePipeStart(ctx.Schema)
		}
		return completeStatementStart(ctx.Schema)
	}

	// 8. Clause-level completion (WHERE / RETURN / YIELD / WITH / ORDER BY / GROUP BY / MATCH pattern)
	return completeByClause(ac, ctx)
}

// Completions builds the full schema-aware autocomplete list for the nGQL code editor.
// Retained for backwards compatibility and unscoped completion callers.
func Completions(schema *model.SpaceSchema) []editor.Completion {
	var items []editor.Completion

	// 1. Live Schema items first
	if schema != nil {
		items = append(items, tagCompletions(schema, nil, false)...)
		for _, tag := range schema.Tags {
			for _, p := range tag.Properties {
				items = append(items, propCompletion("TAG", tag.Name, p, true, schema, false))
			}
		}
		items = append(items, edgeCompletions(schema, nil, false)...)
		for _, edge := range schema.Edges {
			for _, p := range edge.Properties {
				items = append(items, propCompletion("EDGE", edge.Name, p, true, schema, false))
			}
		}
		for _, prop := range schema.AllPropertyNames() {
			items = append(items, editor.Completion{
				Label:  prop,
				Detail: "Property",
				Kind:   editor.CompletionField,
			})
		}
	}

	// 2. Query snippets
	items = append(items, snippetCompletions(schema)...)

	// 3. Built-in functions
	items = append(items, functionCompletions()...)

	// 4. nGQL Keywords
	items = append(items, allKeywordCompletions()...)

	return items
}

func completeDotChain(chain []string, ac *analyzedContext, schema *model.SpaceSchema, explicit bool) []editor.Completion {
	inBT := ac.inBacktick
	last := chain[len(chain)-1]

	// Case A: 2+ level chain ending in a Tag name, e.g. `v.player.`, `$^.player.`, `$$.player.`
	if len(chain) >= 2 && schema != nil {
		if tag := findTag(schema, last); tag != nil {
			return tagPropsOnly(tag, schema, inBT)
		}
		if edge := findEdge(schema, last); edge != nil {
			return edgePropsWithMeta(edge, schema, inBT)
		}
	}

	// Case B: Pipe input variable `$-.`
	if last == "$-" {
		var items []editor.Completion
		for _, alias := range ac.pipeAliases {
			items = append(items, editor.Completion{
				Label:         alias,
				Detail:        "Piped column",
				Kind:          editor.CompletionVariable,
				Insert:        formatIdent(alias, inBT),
				Documentation: fmt.Sprintf("Column `%s` piped from previous YIELD stage", alias),
			})
		}
		return items
	}

	// Case C: Source / Destination vertex `$^.` or `$$.`
	if last == "$^" || last == "$$" {
		if schema == nil {
			return nil
		}
		role := "Source vertex ($^)"
		if last == "$$" {
			role = "Destination vertex ($$)"
		}
		var items []editor.Completion
		for _, tag := range schema.Tags {
			for _, p := range tag.Properties {
				c := propCompletion("TAG", tag.Name, p, true, schema, inBT)
				c.Documentation = fmt.Sprintf("%s · %s", role, c.Documentation)
				items = append(items, c)
			}
		}
		items = append(items, tagCompletions(schema, nil, inBT)...)
		return items
	}

	// Case D: Bound query variable (`v.`, `e.`)
	if binding, ok := ac.bindings[last]; ok {
		switch binding.Kind {
		case varVertex:
			if schema == nil {
				return nil
			}
			var items []editor.Completion
			if len(binding.Types) > 0 {
				// Bound to specific Tag(s): in NebulaGraph MATCH, vertex props are `v.tag.prop`
				for _, tn := range binding.Types {
					if tag := findTag(schema, tn); tag != nil {
						for _, p := range tag.Properties {
							items = append(items, propCompletion("TAG", tag.Name, p, true, schema, inBT))
						}
						items = append(items, tagCompletionItem(*tag, schema, inBT))
					}
				}
				return items
			}
			// Untyped vertex `(n)`: offer all tag.prop and tags
			for _, tag := range schema.Tags {
				for _, p := range tag.Properties {
					items = append(items, propCompletion("TAG", tag.Name, p, true, schema, inBT))
				}
			}
			items = append(items, tagCompletions(schema, nil, inBT)...)
			return items

		case varEdge:
			if schema == nil {
				return nil
			}
			var items []editor.Completion
			seen := map[string]bool{}
			if len(binding.Types) > 0 {
				// Edge properties in MATCH are accessed directly as `e.prop`
				for _, en := range binding.Types {
					if edge := findEdge(schema, en); edge != nil {
						for _, p := range edge.Properties {
							if !seen[p.Field] {
								seen[p.Field] = true
								items = append(items, propCompletion("EDGE", edge.Name, p, false, schema, inBT))
							}
						}
					}
				}
				return items
			}
			// Untyped edge `[e]`: offer all edge properties
			for _, edge := range schema.Edges {
				for _, p := range edge.Properties {
					if !seen[p.Field] {
						seen[p.Field] = true
						items = append(items, propCompletion("EDGE", edge.Name, p, false, schema, inBT))
					}
				}
			}
			return items
		}
	}

	// Case E: Direct Tag name qualifier (`player.`) in LOOKUP / FETCH / WHERE
	if schema != nil {
		if tag := findTag(schema, last); tag != nil {
			return tagPropsOnly(tag, schema, inBT)
		}
		// Case F: Direct Edge type qualifier (`follow.`) in GO / FETCH / LOOKUP
		if edge := findEdge(schema, last); edge != nil {
			return edgePropsWithMeta(edge, schema, inBT)
		}
	}

	// Case G: Unknown qualifier before `.`
	if explicit && schema != nil {
		var items []editor.Completion
		for _, prop := range schema.AllPropertyNames() {
			items = append(items, editor.Completion{
				Label:  prop,
				Detail: "Property",
				Kind:   editor.CompletionField,
				Insert: formatIdent(prop, inBT),
			})
		}
		return items
	}
	return nil
}

func completeInsideDelimiters(ac *analyzedContext, schema *model.SpaceSchema, explicit bool) ([]editor.Completion, bool) {
	if len(ac.delimStack) == 0 {
		return nil, false
	}
	top := ac.delimStack[len(ac.delimStack)-1]
	toks := ac.pipeTokens
	var lastTok stmtToken
	hasLast := len(toks) > 0
	if hasLast {
		lastTok = toks[len(toks)-1]
	}

	// 1. Inside inline property map `{ ... }` of a vertex or edge pattern
	if top.open == '{' && top.isPattern {
		// After `prop:`, user is typing a literal value — don't suggest property names.
		if hasLast && lastTok.kind == tokPunct && lastTok.text == ":" {
			return nil, true
		}
		// Right after `{` or `,`: suggest properties of the enclosing tag(s)/edge(s).
		if hasLast && lastTok.kind == tokPunct && (lastTok.text == "{" || lastTok.text == ",") {
			return inlineMapPropCompletions(top, schema), true
		}
		return nil, true
	}

	// 2. Inside edge pattern `[ ... ]`
	if top.open == '[' && top.isPattern {
		// Right after `:` or `|`: suggest Edge types!
		if hasLast && lastTok.kind == tokPunct && (lastTok.text == ":" || lastTok.text == "|") {
			exclude := make(map[string]bool, len(top.ownerTypes))
			for _, t := range top.ownerTypes {
				exclude[strings.ToLower(t)] = true
			}
			return edgeCompletions(schema, exclude, ac.inBacktick), true
		}
		// Right after `[` (typing a new edge variable name like `[e`): suppress noisy auto-popups.
		if hasLast && lastTok.kind == tokPunct && lastTok.text == "[" {
			if !explicit {
				return nil, true
			}
			return edgeCompletions(schema, nil, ac.inBacktick), true
		}
		return nil, true
	}

	// 3. Inside vertex pattern `( ... )`
	if top.open == '(' && top.isPattern {
		// Right after `:`: suggest Tags!
		if hasLast && lastTok.kind == tokPunct && lastTok.text == ":" {
			exclude := make(map[string]bool, len(top.ownerTypes))
			for _, t := range top.ownerTypes {
				exclude[strings.ToLower(t)] = true
			}
			return tagCompletions(schema, exclude, ac.inBacktick), true
		}
		// Right after `(` (typing a new vertex variable name like `(v`): suppress noisy auto-popups,
		// or on explicit Ctrl+Space offer existing vertex variables for pattern reuse.
		if hasLast && lastTok.kind == tokPunct && lastTok.text == "(" {
			if !explicit {
				return nil, true
			}
			var items []editor.Completion
			for _, b := range ac.bindingList {
				if b.Kind == varVertex {
					items = append(items, editor.Completion{
						Label:  b.Name,
						Detail: "Vertex variable",
						Kind:   editor.CompletionVariable,
					})
				}
			}
			return items, true
		}
		return nil, true
	}

	// 4. Inside DDL column definition: CREATE TAG / CREATE EDGE / ALTER TAG / ALTER EDGE (prop type ...)
	if top.open == '(' && (ac.stmtHead == "CREATE" || ac.stmtHead == "ALTER") {
		if hasLast && lastTok.isIdent() && !keywords[lastTok.upper] && !dataTypes[lastTok.upper] {
			// User just typed a property name -> suggest NebulaGraph data types!
			return dataTypeCompletions(), true
		}
		if hasLast && (dataTypes[lastTok.upper] || (lastTok.kind == tokPunct && lastTok.text == ")")) {
			return keywordSubset([]string{"NOT", "NULL", "DEFAULT", "COMMENT"}), true
		}
	}

	return nil, false
}

func completeAfterTokens(ac *analyzedContext, ctx CompletionContext) ([]editor.Completion, bool) {
	toks := ac.pipeTokens
	n := len(toks)
	if n == 0 {
		return nil, false
	}
	last := toks[n-1]
	var prev stmtToken
	if n >= 2 {
		prev = toks[n-2]
	}
	var prev2 stmtToken
	if n >= 3 {
		prev2 = toks[n-3]
	}

	schema := ctx.Schema
	inBT := ac.inBacktick

	// Multi-token patterns (check most specific first!)

	// DESCRIBE / DESC / DROP / SHOW CREATE / REBUILD TAG INDEX / EDGE INDEX <index_name>
	if n >= 3 && last.kind == tokWord && last.upper == "INDEX" && prev.kind == tokWord {
		if prev.upper == "TAG" && (prev2.upper == "DESCRIBE" || prev2.upper == "DESC" || prev2.upper == "DROP" || prev2.upper == "REBUILD" || prev2.upper == "CREATE") {
			if prev2.upper != "CREATE" || (n >= 4 && toks[n-4].upper == "SHOW") {
				return indexCompletions(schema, "TAG", inBT), true
			}
		}
		if prev.upper == "EDGE" && (prev2.upper == "DESCRIBE" || prev2.upper == "DESC" || prev2.upper == "DROP" || prev2.upper == "REBUILD" || prev2.upper == "CREATE") {
			if prev2.upper != "CREATE" || (n >= 4 && toks[n-4].upper == "SHOW") {
				return indexCompletions(schema, "EDGE", inBT), true
			}
		}
	}

	// DROP SPACE IF EXISTS <space>, DROP TAG IF EXISTS <tag>, DROP EDGE IF EXISTS <edge>
	if n >= 4 && last.upper == "EXISTS" && prev.upper == "IF" {
		switch prev2.upper {
		case "SPACE":
			return spaceCompletions(ctx.Spaces, inBT), true
		case "TAG":
			return tagCompletions(schema, nil, inBT), true
		case "EDGE":
			return edgeCompletions(schema, nil, inBT), true
		}
	}

	// SHOW CREATE ...
	if n >= 2 && prev.upper == "SHOW" && last.upper == "CREATE" {
		return phraseCompletions([][2]string{
			{"SPACE", "Show CREATE SPACE DDL"},
			{"TAG", "Show CREATE TAG DDL"},
			{"EDGE", "Show CREATE EDGE DDL"},
			{"TAG INDEX", "Show CREATE TAG INDEX DDL"},
			{"EDGE INDEX", "Show CREATE EDGE INDEX DDL"},
		}), true
	}

	// SHOW TAG / SHOW EDGE ...
	if n >= 2 && prev.upper == "SHOW" && last.upper == "TAG" {
		return phraseCompletions([][2]string{
			{"INDEXES", "List all tag indexes"},
			{"INDEX STATUS", "Show tag index rebuild status"},
		}), true
	}
	if n >= 2 && prev.upper == "SHOW" && last.upper == "EDGE" {
		return phraseCompletions([][2]string{
			{"INDEXES", "List all edge indexes"},
			{"INDEX STATUS", "Show edge index rebuild status"},
		}), true
	}

	// SUBMIT JOB ...
	if n >= 2 && prev.upper == "SUBMIT" && last.upper == "JOB" {
		return phraseCompletions([][2]string{
			{"STATS", "Collect vertex and edge statistics for SHOW STATS"},
			{"COMPACT", "Compact storage RocksDB files"},
			{"FLUSH", "Flush memtables to disk"},
			{"REBUILD TAG INDEX", "Rebuild a tag secondary index"},
			{"REBUILD EDGE INDEX", "Rebuild an edge secondary index"},
		}), true
	}

	// LOOKUP ON <tag|edge> (prioritize indexed targets!)
	if n >= 2 && prev.upper == "LOOKUP" && last.upper == "ON" {
		return lookupTargetCompletions(schema, inBT), true
	}

	// FETCH PROP ON <tag|edge|*>
	if n >= 2 && (prev.upper == "PROP" || prev.upper == "PROPS") && last.upper == "ON" {
		items := tagCompletions(schema, nil, inBT)
		items = append(items, edgeCompletions(schema, nil, inBT)...)
		items = append(items, editor.Completion{
			Label:         "*",
			Detail:        "All tags",
			Kind:          editor.CompletionConstant,
			Documentation: "Fetch all tags and properties for the specified VID(s)",
		})
		return items, true
	}

	// CREATE TAG INDEX ... ON <tag> / CREATE EDGE INDEX ... ON <edge>
	if last.upper == "ON" && ac.stmtHead == "CREATE" {
		for _, tk := range toks {
			if tk.upper == "TAG" {
				return tagCompletions(schema, nil, inBT), true
			}
			if tk.upper == "EDGE" {
				return edgeCompletions(schema, nil, inBT), true
			}
		}
	}

	// USE <space> | DESCRIBE SPACE <space> | DROP SPACE <space> | ALTER SPACE <space> | SHOW CREATE SPACE <space>
	if last.kind == tokWord && last.upper == "USE" {
		return spaceCompletions(ctx.Spaces, inBT), true
	}
	if n >= 2 && last.upper == "SPACE" && (prev.upper == "DESCRIBE" || prev.upper == "DESC" || prev.upper == "DROP" || prev.upper == "ALTER" || prev.upper == "CREATE") {
		if prev.upper != "CREATE" || (n >= 3 && prev2.upper == "SHOW") {
			return spaceCompletions(ctx.Spaces, inBT), true
		}
	}

	// DESCRIBE/DESC/DROP/ALTER/INSERT VERTEX/SHOW CREATE TAG <tag>
	if n >= 2 && last.upper == "TAG" && (prev.upper == "DESCRIBE" || prev.upper == "DESC" || prev.upper == "DROP" || prev.upper == "ALTER" || prev.upper == "DELETE") {
		return tagCompletions(schema, nil, inBT), true
	}
	if n >= 3 && last.upper == "TAG" && prev.upper == "CREATE" && prev2.upper == "SHOW" {
		return tagCompletions(schema, nil, inBT), true
	}
	if n >= 2 && last.upper == "VERTEX" && (prev.upper == "INSERT" || prev.upper == "UPDATE" || prev.upper == "UPSERT") {
		return tagCompletions(schema, nil, inBT), true
	}

	// DESCRIBE/DESC/DROP/ALTER/INSERT/SHOW CREATE EDGE <edge>
	if n >= 2 && last.upper == "EDGE" && (prev.upper == "DESCRIBE" || prev.upper == "DESC" || prev.upper == "DROP" || prev.upper == "ALTER" || prev.upper == "INSERT" || prev.upper == "UPDATE" || prev.upper == "UPSERT" || prev.upper == "DELETE") {
		return edgeCompletions(schema, nil, inBT), true
	}
	if n >= 3 && last.upper == "EDGE" && prev.upper == "CREATE" && prev2.upper == "SHOW" {
		return edgeCompletions(schema, nil, inBT), true
	}

	// OVER <edge>, <edge2> | GET SUBGRAPH ... BOTH/IN/OUT <edge>
	if last.kind == tokWord && last.upper == "OVER" {
		return overEdgeCompletions(schema, nil, inBT), true
	}
	if last.kind == tokWord && (last.upper == "BOTH" || last.upper == "OUT" || last.upper == "IN") && ac.stmtHead == "GET" {
		return overEdgeCompletions(schema, nil, inBT), true
	}
	// Comma inside OVER edge1, <edge2>
	if last.kind == tokPunct && last.text == "," && ac.clause == "OVER" {
		exclude := make(map[string]bool, len(ac.overEdges))
		for _, e := range ac.overEdges {
			exclude[strings.ToLower(e)] = true
		}
		return edgeCompletions(schema, exclude, inBT), true
	}
	// Right after an edge name in OVER clause: suggest direction & next clauses
	if ac.clause == "OVER" && last.isIdent() && schema != nil && findEdge(schema, last.value) != nil {
		return keywordSubset([]string{"REVERSELY", "BIDIRECT", "WHERE", "YIELD", "UPTO"}), true
	}

	// Single-keyword subcommand followers
	if last.kind == tokWord {
		switch last.upper {
		case "SHOW":
			items := make([]editor.Completion, 0, len(showSubcommands))
			for _, sc := range showSubcommands {
				items = append(items, editor.Completion{
					Label:         sc.label,
					Detail:        "SHOW",
					Kind:          editor.CompletionKeyword,
					Documentation: sc.doc,
				})
			}
			return items, true

		case "DESCRIBE", "DESC":
			// Only treat DESC as DESCRIBE when at statement start (not ORDER BY x DESC)
			if n == 1 {
				return phraseCompletions([][2]string{
					{"TAG", "Describe properties of a tag"},
					{"EDGE", "Describe properties of an edge type"},
					{"SPACE", "Describe graph space configuration"},
					{"TAG INDEX", "Describe a tag index"},
					{"EDGE INDEX", "Describe an edge index"},
				}), true
			}

		case "CREATE":
			return phraseCompletions([][2]string{
				{"TAG", "Create a new vertex tag schema"},
				{"EDGE", "Create a new edge type schema"},
				{"TAG INDEX", "Create a secondary index on a tag"},
				{"EDGE INDEX", "Create a secondary index on an edge type"},
				{"SPACE", "Create a new graph space"},
			}), true

		case "DROP":
			return phraseCompletions([][2]string{
				{"TAG", "Drop a vertex tag"},
				{"EDGE", "Drop an edge type"},
				{"TAG INDEX", "Drop a tag index"},
				{"EDGE INDEX", "Drop an edge index"},
				{"SPACE", "Drop a graph space"},
			}), true

		case "ALTER":
			return phraseCompletions([][2]string{
				{"TAG", "Alter tag properties"},
				{"EDGE", "Alter edge type properties"},
				{"SPACE", "Alter graph space settings"},
			}), true

		case "REBUILD":
			return phraseCompletions([][2]string{
				{"TAG INDEX", "Rebuild tag index for existing vertices"},
				{"EDGE INDEX", "Rebuild edge index for existing edges"},
			}), true

		case "GET":
			return phraseCompletions([][2]string{
				{"SUBGRAPH", "Fetch an N-hop subgraph around seed VID(s)"},
				{"SUBGRAPH WITH PROP", "Fetch an N-hop subgraph including all vertex/edge properties"},
			}), true

		case "FIND":
			return phraseCompletions([][2]string{
				{"SHORTEST PATH", "Find shortest path(s) between VIDs"},
				{"ALL PATH", "Find all paths up to N steps between VIDs"},
				{"NOLOOP PATH", "Find non-looping paths between VIDs"},
			}), true

		case "FETCH":
			return phraseCompletions([][2]string{
				{"PROP ON", "Fetch properties for explicit vertex or edge IDs"},
			}), true

		case "LOOKUP":
			return phraseCompletions([][2]string{
				{"ON", "Scan a tag or edge type using a secondary index"},
			}), true

		case "OPTIONAL":
			return phraseCompletions([][2]string{
				{"MATCH", "Match optional graph pattern (left outer join)"},
			}), true

		case "ORDER", "GROUP":
			return phraseCompletions([][2]string{
				{"BY", "Specify sorting or grouping expression"},
			}), true

		case "INSERT":
			return phraseCompletions([][2]string{
				{"VERTEX", "Insert one or more vertices with tag properties"},
				{"EDGE", "Insert one or more directed edges"},
			}), true

		case "UPDATE", "UPSERT":
			return phraseCompletions([][2]string{
				{"VERTEX", "Update/upsert vertex properties by VID"},
				{"EDGE", "Update/upsert edge properties by src -> dst"},
			}), true

		case "DELETE":
			return phraseCompletions([][2]string{
				{"VERTEX", "Delete vertex(es) by VID (add WITH EDGE to remove incident edges)"},
				{"EDGE", "Delete edge(s) by src -> dst"},
				{"TAG", "Delete specific tag(s) from a vertex"},
			}), true

		case "SUBMIT":
			return phraseCompletions([][2]string{
				{"JOB", "Submit an asynchronous cluster job"},
			}), true

		case "EXPLAIN", "PROFILE":
			if n == 1 {
				return completeStatementStart(schema), true
			}

		case "FROM", "TO":
			if ac.hasPipe {
				var items []editor.Completion
				for _, alias := range ac.pipeAliases {
					items = append(items, editor.Completion{
						Label:         "$-." + alias,
						Detail:        "Piped VID",
						Kind:          editor.CompletionVariable,
						Documentation: fmt.Sprintf("Use piped column `%s` from previous stage", alias),
					})
				}
				items = append(items, editor.Completion{
					Label:         "$-",
					Detail:        "Pipe input",
					Kind:          editor.CompletionVariable,
					Documentation: "Reference columns yielded from the previous piped query",
				})
				items = append(items, vidCompletions(ctx.SampleVIDs, schema, true)...)
				return items, true
			}
			if len(ctx.SampleVIDs) > 0 {
				return vidCompletions(ctx.SampleVIDs, schema, true), true
			}
		}
	}

	return nil, false
}

func completeStatementStart(schema *model.SpaceSchema) []editor.Completion {
	var items []editor.Completion
	// 1. Schema-adapted query snippets first (huge productivity boost!)
	items = append(items, snippetCompletions(schema)...)

	// 2. Top-level nGQL statement starters
	starters := []string{
		"MATCH", "OPTIONAL", "GO", "GET", "FIND", "FETCH", "LOOKUP",
		"SHOW", "USE", "DESCRIBE", "EXPLAIN", "PROFILE",
		"CREATE", "INSERT", "UPDATE", "UPSERT", "DELETE", "ALTER", "DROP", "SUBMIT", "REBUILD",
	}
	items = append(items, keywordSubset(starters)...)
	return items
}

func completePipeStart(schema *model.SpaceSchema) []editor.Completion {
	items := phraseCompletions([][2]string{
		{"YIELD", "Project or filter piped columns ($-.col)"},
		{"ORDER BY", "Sort piped rows"},
		{"LIMIT", "Limit piped row count"},
		{"GROUP BY", "Group and aggregate piped rows"},
		{"GO", "Continue graph traversal from piped VIDs (FROM $-.col)"},
		{"FETCH PROP ON", "Fetch vertex/edge properties for piped VIDs"},
		{"GET SUBGRAPH", "Fetch neighborhood subgraph around piped VIDs"},
		{"FIND SHORTEST PATH", "Compute shortest path using piped VIDs"},
		{"DELETE VERTEX", "Delete piped vertices"},
	})
	// Also allow snippets after pipe
	items = append(items, snippetCompletions(schema)...)
	return items
}

func completeByClause(ac *analyzedContext, ctx CompletionContext) []editor.Completion {
	schema := ctx.Schema
	inBT := ac.inBacktick
	var items []editor.Completion

	switch ac.clause {
	case "WHERE", "RETURN", "YIELD", "WITH", "ORDER_BY", "GROUP_BY", "SET", "WHEN":
		// 1. Bound query variables & smart property shortcuts!
		items = append(items, variableAndShortcutCompletions(ac, schema, inBT)...)

		// 2. Built-in graph & scalar functions
		items = append(items, functionCompletions()...)

		// 3. Relevant schema items (tags, edges, tag.prop, edge.prop)
		if schema != nil {
			// If LOOKUP ON / FETCH PROP ON has a known target, prioritize its properties!
			if ac.onTarget != "" {
				if tag := findTag(schema, ac.onTarget); tag != nil {
					for _, p := range tag.Properties {
						items = append(items, propCompletion("TAG", tag.Name, p, true, schema, inBT))
					}
				} else if edge := findEdge(schema, ac.onTarget); edge != nil {
					for _, p := range edge.Properties {
						items = append(items, propCompletion("EDGE", edge.Name, p, true, schema, inBT))
					}
				}
			}
			// If GO OVER has known edges, prioritize their properties!
			for _, en := range ac.overEdges {
				if edge := findEdge(schema, en); edge != nil {
					for _, p := range edge.Properties {
						items = append(items, propCompletion("EDGE", edge.Name, p, true, schema, inBT))
					}
				}
			}
			items = append(items, tagCompletions(schema, nil, inBT)...)
			items = append(items, edgeCompletions(schema, nil, inBT)...)
		}

		// 4. Clause-appropriate keywords
		switch ac.clause {
		case "WHERE":
			items = append(items, keywordSubset([]string{
				"AND", "OR", "XOR", "NOT", "IN", "IS", "CONTAINS", "STARTS", "ENDS", "WITH",
				"EXISTS", "RETURN", "YIELD", "ORDER", "LIMIT", "SKIP",
			})...)
			items = append(items, constantCompletions()...)
		case "RETURN", "YIELD", "WITH":
			items = append(items, keywordSubset([]string{
				"AS", "DISTINCT", "ORDER", "BY", "SKIP", "LIMIT", "WHERE", "UNION", "ALL",
				"VERTICES", "EDGES", "CASE", "WHEN", "THEN", "ELSE", "END",
			})...)
			items = append(items, nativeYieldObjectCompletions(ac)...)
		case "ORDER_BY":
			items = append(items, keywordSubset([]string{"ASC", "DESC", "SKIP", "LIMIT"})...)
		case "GROUP_BY":
			items = append(items, keywordSubset([]string{"YIELD", "AS"})...)
		}
		return dedupeCompletions(items)

	case "MATCH":
		// Right after `MATCH` with no pattern yet: don't spam while user types `p=(...)`,
		// unless explicitly requested via Ctrl+Space.
		if len(ac.pipeTokens) > 0 && ac.pipeTokens[len(ac.pipeTokens)-1].upper == "MATCH" {
			if !ctx.Explicit {
				return nil
			}
			return snippetCompletions(schema)
		}
		// After completing part of a MATCH pattern (e.g. `MATCH (v:player)-[e:follow]->(n) |`)
		items = append(items, keywordSubset([]string{
			"WHERE", "RETURN", "WITH", "OPTIONAL", "UNWIND", "ORDER", "LIMIT", "SKIP",
		})...)
		return dedupeCompletions(items)

	case "GO":
		items = append(items, keywordSubset([]string{
			"STEPS", "STEP", "TO", "FROM", "OVER", "REVERSELY", "BIDIRECT", "WHERE", "YIELD", "SAMPLE",
		})...)
		items = append(items, edgeCompletions(schema, nil, inBT)...)
		return dedupeCompletions(items)

	case "GET":
		items = append(items, keywordSubset([]string{
			"SUBGRAPH", "WITH", "PROP", "STEPS", "FROM", "BOTH", "IN", "OUT", "YIELD", "VERTICES", "EDGES",
		})...)
		items = append(items, edgeCompletions(schema, nil, inBT)...)
		return dedupeCompletions(items)

	case "FIND":
		items = append(items, keywordSubset([]string{
			"SHORTEST", "ALL", "NOLOOP", "PATH", "WITH", "PROP", "FROM", "TO", "OVER", "REVERSELY", "BIDIRECT", "UPTO", "STEPS", "YIELD",
		})...)
		items = append(items, edgeCompletions(schema, nil, inBT)...)
		return dedupeCompletions(items)

	case "LIMIT", "SKIP":
		return keywordSubset([]string{"OFFSET", "LIMIT", "SKIP"})
	}

	// Default fallback: variables + schema + snippets + functions + keywords
	items = append(items, variableAndShortcutCompletions(ac, schema, inBT)...)
	items = append(items, tagCompletions(schema, nil, inBT)...)
	items = append(items, edgeCompletions(schema, nil, inBT)...)
	items = append(items, functionCompletions()...)
	items = append(items, allKeywordCompletions()...)
	return dedupeCompletions(items)
}

func variableAndShortcutCompletions(ac *analyzedContext, schema *model.SpaceSchema, inBT bool) []editor.Completion {
	var items []editor.Completion

	// Bound variables from MATCH / AS aliases
	for _, b := range ac.bindingList {
		detail := "Variable"
		doc := fmt.Sprintf("Query variable `%s`", b.Name)
		switch b.Kind {
		case varVertex:
			if len(b.Types) > 0 {
				detail = "Vertex :" + strings.Join(b.Types, ":")
				doc = fmt.Sprintf("Vertex variable `%s` (tag: %s)", b.Name, strings.Join(b.Types, ", "))
			} else {
				detail = "Vertex"
				doc = fmt.Sprintf("Vertex variable `%s`", b.Name)
			}
		case varEdge:
			if len(b.Types) > 0 {
				detail = "Edge :" + strings.Join(b.Types, "|")
				doc = fmt.Sprintf("Edge variable `%s` (type: %s)", b.Name, strings.Join(b.Types, ", "))
			} else {
				detail = "Edge"
				doc = fmt.Sprintf("Edge variable `%s`", b.Name)
			}
		case varPath:
			detail = "Path"
			doc = fmt.Sprintf("Path variable `%s`", b.Name)
		case varAlias:
			detail = "Alias"
			doc = fmt.Sprintf("Column alias `%s`", b.Name)
		}
		items = append(items, editor.Completion{
			Label:         b.Name,
			Detail:        detail,
			Kind:          editor.CompletionVariable,
			Insert:        formatIdent(b.Name, inBT),
			Documentation: doc,
		})
	}

	// Smart property shortcuts for bound variables (e.g., `v.player.name`, `e.degree`)
	if schema != nil {
		for _, b := range ac.bindingList {
			switch b.Kind {
			case varVertex:
				for _, tn := range b.Types {
					if tag := findTag(schema, tn); tag != nil {
						for _, p := range tag.Properties {
							label := b.Name + "." + tag.Name + "." + p.Field
							items = append(items, editor.Completion{
								Label:         label,
								Detail:        propDetail("TAG", tag.Name, p, schema),
								Kind:          editor.CompletionField,
								Documentation: propDoc("TAG", tag.Name, p, schema),
							})
						}
					}
				}
			case varEdge:
				for _, en := range b.Types {
					if edge := findEdge(schema, en); edge != nil {
						for _, p := range edge.Properties {
							label := b.Name + "." + p.Field
							items = append(items, editor.Completion{
								Label:         label,
								Detail:        propDetail("EDGE", edge.Name, p, schema),
								Kind:          editor.CompletionField,
								Documentation: propDoc("EDGE", edge.Name, p, schema),
							})
						}
					}
				}
			}
		}
	}

	// Native nGQL special variables when in GO / pipe context
	if ac.stmtHead == "GO" {
		items = append(items,
			editor.Completion{Label: "$^", Detail: "Source vertex", Kind: editor.CompletionVariable, Documentation: "Source vertex of the traversed edge in GO"},
			editor.Completion{Label: "$$", Detail: "Dest vertex", Kind: editor.CompletionVariable, Documentation: "Destination vertex of the traversed edge in GO"},
			editor.Completion{Label: "edge", Detail: "Edge object", Kind: editor.CompletionVariable, Documentation: "Current traversed edge object (use with YIELD edge AS e, src(edge), dst(edge))"},
		)
	}
	if ac.hasPipe {
		for _, alias := range ac.pipeAliases {
			items = append(items, editor.Completion{
				Label:         "$-." + alias,
				Detail:        "Piped column",
				Kind:          editor.CompletionVariable,
				Documentation: fmt.Sprintf("Column `%s` piped from previous YIELD stage", alias),
			})
		}
	}

	return items
}

func nativeYieldObjectCompletions(ac *analyzedContext) []editor.Completion {
	switch ac.stmtHead {
	case "FETCH", "LOOKUP":
		return []editor.Completion{
			{Label: "vertex", Detail: "Vertex object", Kind: editor.CompletionVariable, Documentation: "Yields the full vertex object (visualizable in Graph View)"},
			{Label: "edge", Detail: "Edge object", Kind: editor.CompletionVariable, Documentation: "Yields the full edge object (visualizable in Graph View)"},
		}
	case "FIND":
		return []editor.Completion{
			{Label: "path", Detail: "Path object", Kind: editor.CompletionVariable, Documentation: "Yields the matched path object (YIELD path AS p)"},
		}
	}
	return nil
}

func inlineMapPropCompletions(frame delimFrame, schema *model.SpaceSchema) []editor.Completion {
	if schema == nil {
		return nil
	}
	var items []editor.Completion
	seen := map[string]bool{}
	addProp := func(kind, owner string, p model.PropertySchema) {
		if seen[p.Field] {
			return
		}
		seen[p.Field] = true
		comp := editor.Completion{
			Label:         p.Field,
			Detail:        propDetail(kind, owner, p, schema),
			Kind:          editor.CompletionField,
			Documentation: propDoc(kind, owner, p, schema),
		}
		if strings.Contains(strings.ToLower(p.Type), "string") {
			if snip, err := editor.ParseSnippet(p.Field + `: "${1}"$0`); err == nil {
				comp.Snippet = snip
			} else {
				comp.Insert = p.Field + `: ""`
			}
		} else {
			if snip, err := editor.ParseSnippet(p.Field + `: ${1}$0`); err == nil {
				comp.Snippet = snip
			} else {
				comp.Insert = p.Field + ": "
			}
		}
		items = append(items, comp)
	}

	if frame.ownerIsEdg {
		if len(frame.ownerTypes) > 0 {
			for _, en := range frame.ownerTypes {
				if edge := findEdge(schema, en); edge != nil {
					for _, p := range edge.Properties {
						addProp("EDGE", edge.Name, p)
					}
				}
			}
		} else {
			for _, edge := range schema.Edges {
				for _, p := range edge.Properties {
					addProp("EDGE", edge.Name, p)
				}
			}
		}
		return items
	}

	if len(frame.ownerTypes) > 0 {
		for _, tn := range frame.ownerTypes {
			if tag := findTag(schema, tn); tag != nil {
				for _, p := range tag.Properties {
					addProp("TAG", tag.Name, p)
				}
			}
		}
	} else {
		for _, tag := range schema.Tags {
			for _, p := range tag.Properties {
				addProp("TAG", tag.Name, p)
			}
		}
	}
	return items
}

func isVIDStringPosition(toks []stmtToken) bool {
	n := len(toks)
	if n == 0 {
		return false
	}
	last := toks[n-1]
	// Direct: FROM "vid", TO "vid"
	if last.kind == tokWord && (last.upper == "FROM" || last.upper == "TO") {
		return true
	}
	// FETCH PROP ON tag "vid"
	if n >= 3 && last.isIdent() && toks[n-2].upper == "ON" && (toks[n-3].upper == "PROP" || toks[n-3].upper == "PROPS") {
		return true
	}
	// Comma in VID list: FROM "v1", "v2" or FETCH PROP ON tag "v1", "v2"
	if last.kind == tokPunct && last.text == "," {
		for k := n - 2; k >= 0; k-- {
			if toks[k].kind == tokWord {
				switch toks[k].upper {
				case "FROM", "TO", "PROP", "PROPS":
					return true
				case "WHERE", "YIELD", "RETURN", "OVER", "SET", "VALUES":
					return false
				}
			}
		}
	}
	return false
}

func vidCompletions(vids []string, schema *model.SpaceSchema, includeQuotes bool) []editor.Completion {
	if len(vids) == 0 {
		return nil
	}
	isInt := isIntVIDSchema(schema)
	var items []editor.Completion
	for _, vid := range vids {
		ins := vid
		label := vid
		if includeQuotes {
			ins = model.FormatNGQLVID(vid, isInt)
			label = ins
		}
		items = append(items, editor.Completion{
			Label:         label,
			Detail:        "Canvas VID",
			Kind:          editor.CompletionConstant,
			Insert:        ins,
			FilterText:    vid,
			Documentation: fmt.Sprintf("Vertex ID `%s` from active graph canvas", vid),
		})
	}
	return items
}

func spaceCompletions(spaces []string, inBT bool) []editor.Completion {
	var items []editor.Completion
	for _, sp := range spaces {
		items = append(items, editor.Completion{
			Label:         sp,
			Detail:        "Space",
			Kind:          editor.CompletionModule,
			Insert:        formatIdent(sp, inBT),
			Documentation: fmt.Sprintf("Graph Space `%s`", sp),
		})
	}
	return items
}

func tagCompletions(schema *model.SpaceSchema, exclude map[string]bool, inBT bool) []editor.Completion {
	if schema == nil {
		return nil
	}
	var items []editor.Completion
	for _, tag := range schema.Tags {
		if exclude != nil && exclude[strings.ToLower(tag.Name)] {
			continue
		}
		items = append(items, tagCompletionItem(tag, schema, inBT))
	}
	return items
}

func tagCompletionItem(tag model.TagSchema, schema *model.SpaceSchema, inBT bool) editor.Completion {
	var fields []string
	for _, p := range tag.Properties {
		fields = append(fields, fmt.Sprintf("%s: %s", p.Field, p.Type))
	}
	detail := "Tag"
	if hasAnyIndex(schema, "TAG", tag.Name) {
		detail = "Tag · indexed"
	}
	doc := fmt.Sprintf("Tag `%s` in space `%s`", tag.Name, schema.Space)
	if tag.Count > 0 {
		doc += fmt.Sprintf(" (%d vertices)", tag.Count)
	}
	if len(fields) > 0 {
		doc += "\nProperties: " + strings.Join(fields, ", ")
	}
	return editor.Completion{
		Label:         tag.Name,
		Detail:        detail,
		Kind:          editor.CompletionType,
		Insert:        formatIdent(tag.Name, inBT),
		Documentation: doc,
	}
}

func edgeCompletions(schema *model.SpaceSchema, exclude map[string]bool, inBT bool) []editor.Completion {
	if schema == nil {
		return nil
	}
	var items []editor.Completion
	for _, edge := range schema.Edges {
		if exclude != nil && exclude[strings.ToLower(edge.Name)] {
			continue
		}
		items = append(items, edgeCompletionItem(edge, schema, inBT))
	}
	return items
}

func edgeCompletionItem(edge model.EdgeSchema, schema *model.SpaceSchema, inBT bool) editor.Completion {
	var fields []string
	for _, p := range edge.Properties {
		fields = append(fields, fmt.Sprintf("%s: %s", p.Field, p.Type))
	}
	detail := "Edge"
	if hasAnyIndex(schema, "EDGE", edge.Name) {
		detail = "Edge · indexed"
	}
	doc := fmt.Sprintf("Edge Type `%s` in space `%s`", edge.Name, schema.Space)
	if edge.Count > 0 {
		doc += fmt.Sprintf(" (%d edges)", edge.Count)
	}
	if len(fields) > 0 {
		doc += "\nProperties: " + strings.Join(fields, ", ")
	}
	return editor.Completion{
		Label:         edge.Name,
		Detail:        detail,
		Kind:          editor.CompletionModule,
		Insert:        formatIdent(edge.Name, inBT),
		Documentation: doc,
	}
}

func overEdgeCompletions(schema *model.SpaceSchema, exclude map[string]bool, inBT bool) []editor.Completion {
	items := edgeCompletions(schema, exclude, inBT)
	items = append(items, editor.Completion{
		Label:         "*",
		Detail:        "All edges",
		Kind:          editor.CompletionConstant,
		Documentation: "Traverse across all edge types in the current space",
	})
	return items
}

func lookupTargetCompletions(schema *model.SpaceSchema, inBT bool) []editor.Completion {
	if schema == nil {
		return nil
	}
	var indexed, unindexed []editor.Completion
	for _, tag := range schema.Tags {
		c := tagCompletionItem(tag, schema, inBT)
		if hasAnyIndex(schema, "TAG", tag.Name) {
			indexed = append(indexed, c)
		} else {
			unindexed = append(unindexed, c)
		}
	}
	for _, edge := range schema.Edges {
		c := edgeCompletionItem(edge, schema, inBT)
		if hasAnyIndex(schema, "EDGE", edge.Name) {
			indexed = append(indexed, c)
		} else {
			unindexed = append(unindexed, c)
		}
	}
	return append(indexed, unindexed...)
}

func indexCompletions(schema *model.SpaceSchema, kind string, inBT bool) []editor.Completion {
	if schema == nil {
		return nil
	}
	var items []editor.Completion
	for _, idx := range schema.Indexes {
		if !strings.EqualFold(idx.Kind, kind) {
			continue
		}
		detail := fmt.Sprintf("%s Index (%s)", strings.ToUpper(idx.Kind), idx.Target)
		doc := fmt.Sprintf("Index `%s` on %s `%s`", idx.Name, idx.Kind, idx.Target)
		if len(idx.Fields) > 0 {
			doc += "\nFields: " + strings.Join(idx.Fields, ", ")
		}
		items = append(items, editor.Completion{
			Label:         idx.Name,
			Detail:        detail,
			Kind:          editor.CompletionConstant,
			Insert:        formatIdent(idx.Name, inBT),
			Documentation: doc,
		})
	}
	return items
}

func tagPropsOnly(tag *model.TagSchema, schema *model.SpaceSchema, inBT bool) []editor.Completion {
	items := make([]editor.Completion, 0, len(tag.Properties))
	for _, p := range tag.Properties {
		items = append(items, propCompletion("TAG", tag.Name, p, false, schema, inBT))
	}
	return items
}

func edgePropsWithMeta(edge *model.EdgeSchema, schema *model.SpaceSchema, inBT bool) []editor.Completion {
	items := make([]editor.Completion, 0, len(edge.Properties)+4)
	for _, p := range edge.Properties {
		items = append(items, propCompletion("EDGE", edge.Name, p, false, schema, inBT))
	}
	// Built-in NebulaGraph edge system properties
	items = append(items,
		editor.Completion{Label: "_src", Detail: "VID · meta", Kind: editor.CompletionField, Documentation: "Source vertex ID of the edge"},
		editor.Completion{Label: "_dst", Detail: "VID · meta", Kind: editor.CompletionField, Documentation: "Destination vertex ID of the edge"},
		editor.Completion{Label: "_rank", Detail: "int64 · meta", Kind: editor.CompletionField, Documentation: "Ranking value of the edge"},
		editor.Completion{Label: "_type", Detail: "int32 · meta", Kind: editor.CompletionField, Documentation: "Internal numeric edge type ID"},
	)
	return items
}

func propCompletion(kind, owner string, p model.PropertySchema, qualified bool, schema *model.SpaceSchema, inBT bool) editor.Completion {
	label := p.Field
	ins := formatIdent(p.Field, inBT)
	if qualified {
		label = owner + "." + p.Field
		ins = formatIdent(owner, false) + "." + formatIdent(p.Field, false)
	}
	return editor.Completion{
		Label:         label,
		Detail:        propDetail(kind, owner, p, schema),
		Kind:          editor.CompletionField,
		Insert:        ins,
		Documentation: propDoc(kind, owner, p, schema),
	}
}

func propDetail(kind, owner string, p model.PropertySchema, schema *model.SpaceSchema) string {
	detail := p.Type
	if detail == "" {
		detail = "Property"
	}
	if schema != nil && len(schema.FieldIndexes(kind, owner, p.Field)) > 0 {
		detail += " · indexed"
	}
	return detail
}

func propDoc(kind, owner string, p model.PropertySchema, schema *model.SpaceSchema) string {
	doc := fmt.Sprintf("Property `%s.%s` (%s)", owner, p.Field, p.Type)
	if p.Default != "" {
		doc += fmt.Sprintf("\nDefault: %s", p.Default)
	}
	if p.Comment != "" {
		doc += "\n" + p.Comment
	}
	if schema != nil {
		if idxs := schema.FieldIndexes(kind, owner, p.Field); len(idxs) > 0 {
			var names []string
			for _, m := range idxs {
				names = append(names, m.IndexName)
			}
			doc += "\nIndexed by: " + strings.Join(names, ", ")
		}
	}
	return doc
}

func snippetCompletions(schema *model.SpaceSchema) []editor.Completion {
	defs := buildSnippets(schema)
	items := make([]editor.Completion, 0, len(defs))
	for _, s := range defs {
		comp := editor.Completion{
			Label:         s.label,
			Detail:        s.detail,
			Kind:          editor.CompletionSnippet,
			Documentation: s.doc,
		}
		if snip, err := editor.ParseSnippet(s.body); err == nil {
			comp.Snippet = snip
		} else {
			comp.Insert = s.body
		}
		items = append(items, comp)
	}
	return items
}

func functionCompletions() []editor.Completion {
	items := make([]editor.Completion, 0, len(funcCatalog))
	for _, fn := range funcCatalog {
		comp := editor.Completion{
			Label:         fn.name,
			Detail:        "fn",
			Kind:          editor.CompletionFunction,
			Documentation: fn.signature + "\n" + fn.doc,
		}
		if snip, err := editor.ParseSnippet(fn.snippet); err == nil {
			comp.Snippet = snip
		} else {
			comp.Insert = fn.name + "()"
		}
		items = append(items, comp)
	}
	return items
}

func dataTypeCompletions() []editor.Completion {
	items := make([]editor.Completion, 0, len(dataTypeDefs))
	for _, dt := range dataTypeDefs {
		comp := editor.Completion{
			Label:         dt.label,
			Detail:        "Type",
			Kind:          editor.CompletionType,
			Documentation: dt.doc,
		}
		if dt.snippet != "" {
			if snip, err := editor.ParseSnippet(dt.snippet); err == nil {
				comp.Snippet = snip
			}
		}
		items = append(items, comp)
	}
	return items
}

func keywordSubset(list []string) []editor.Completion {
	items := make([]editor.Completion, 0, len(list))
	for _, kw := range list {
		items = append(items, editor.Completion{
			Label:         kw,
			Detail:        "nGQL",
			Kind:          editor.CompletionKeyword,
			Documentation: keywordDocs[kw],
		})
	}
	return items
}

func allKeywordCompletions() []editor.Completion {
	kwList := make([]string, 0, len(keywords))
	for kw := range keywords {
		kwList = append(kwList, kw)
	}
	sort.Strings(kwList)
	return keywordSubset(kwList)
}

func constantCompletions() []editor.Completion {
	return []editor.Completion{
		{Label: "TRUE", Detail: "bool", Kind: editor.CompletionConstant},
		{Label: "FALSE", Detail: "bool", Kind: editor.CompletionConstant},
		{Label: "NULL", Detail: "null", Kind: editor.CompletionConstant},
	}
}

func phraseCompletions(pairs [][2]string) []editor.Completion {
	items := make([]editor.Completion, 0, len(pairs))
	for _, p := range pairs {
		items = append(items, editor.Completion{
			Label:         p[0],
			Detail:        "nGQL",
			Kind:          editor.CompletionKeyword,
			Documentation: p[1],
		})
	}
	return items
}

func dedupeCompletions(items []editor.Completion) []editor.Completion {
	seen := make(map[string]bool, len(items))
	out := make([]editor.Completion, 0, len(items))
	for _, it := range items {
		key := fmt.Sprintf("%d:%s", it.Kind, it.Label)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, it)
	}
	return out
}

func findTag(schema *model.SpaceSchema, name string) *model.TagSchema {
	if schema == nil || name == "" {
		return nil
	}
	for i := range schema.Tags {
		if strings.EqualFold(schema.Tags[i].Name, name) {
			return &schema.Tags[i]
		}
	}
	return nil
}

func findEdge(schema *model.SpaceSchema, name string) *model.EdgeSchema {
	if schema == nil || name == "" {
		return nil
	}
	for i := range schema.Edges {
		if strings.EqualFold(schema.Edges[i].Name, name) {
			return &schema.Edges[i]
		}
	}
	return nil
}

func hasAnyIndex(schema *model.SpaceSchema, kind, target string) bool {
	if schema == nil {
		return false
	}
	for _, idx := range schema.Indexes {
		if strings.EqualFold(idx.Kind, kind) && strings.EqualFold(idx.Target, target) {
			return true
		}
	}
	return false
}

// formatIdent wraps reserved keywords or non-identifier names in backticks unless already inside backticks.
func formatIdent(name string, insideBacktick bool) string {
	if insideBacktick || name == "" {
		return name
	}
	needsQuote := keywords[strings.ToUpper(name)]
	if !needsQuote {
		for i := 0; i < len(name); i++ {
			if (i == 0 && !isIdentStart(name[i])) || (i > 0 && !isIdentPart(name[i])) {
				needsQuote = true
				break
			}
		}
	}
	if needsQuote {
		return "`" + name + "`"
	}
	return name
}

// HoverAt returns rich hover documentation at a byte offset in src, supporting
// qualified properties (v.player.age, player.age, follow.degree), bound query variables,
// spaces, tags/edges with index info, functions, and nGQL keywords.
func HoverAt(src string, offset int, schema *model.SpaceSchema, spaces []string) (doc string, start, end int, ok bool) {
	if offset < 0 || offset > len(src) {
		return "", 0, 0, false
	}
	// First locate the word directly under offset
	start = offset
	for start > 0 && isIdentPart(src[start-1]) {
		start--
	}
	end = offset
	for end < len(src) && isIdentPart(src[end]) {
		end++
	}
	if start == end {
		return "", 0, 0, false
	}
	word := src[start:end]

	// Check if word is part of a dotted chain like `player.age` or `v.player.age`
	chainStart := start
	for chainStart >= 2 && src[chainStart-1] == '.' {
		qStart := chainStart - 1
		for qStart > 0 && isIdentPart(src[qStart-1]) {
			qStart--
		}
		if qStart == chainStart-1 {
			break
		}
		chainStart = qStart
	}
	if chainStart < start && schema != nil {
		chainParts := strings.Split(src[chainStart:end], ".")
		if len(chainParts) >= 2 {
			owner := chainParts[len(chainParts)-2]
			field := chainParts[len(chainParts)-1]
			if tag := findTag(schema, owner); tag != nil {
				for _, p := range tag.Properties {
					if strings.EqualFold(p.Field, field) {
						return propDoc("TAG", tag.Name, p, schema), chainStart, end, true
					}
				}
			}
			if edge := findEdge(schema, owner); edge != nil {
				for _, p := range edge.Properties {
					if strings.EqualFold(p.Field, field) {
						return propDoc("EDGE", edge.Name, p, schema), chainStart, end, true
					}
				}
			}
			// Also resolve `e.degree` where `e` is a bound edge variable
			ac := analyzeContext(src, offset, "")
			if b, exists := ac.bindings[owner]; exists && b.Kind == varEdge {
				for _, en := range b.Types {
					if edge := findEdge(schema, en); edge != nil {
						for _, p := range edge.Properties {
							if strings.EqualFold(p.Field, field) {
								return propDoc("EDGE", edge.Name, p, schema), chainStart, end, true
							}
						}
					}
				}
			}
		}
	}

	// Check schema tags, edges, functions, keywords
	if d, found := HoverDoc(word, schema); found {
		return d, start, end, true
	}

	// Check bound query variables
	ac := analyzeContext(src, offset, "")
	if b, exists := ac.bindings[word]; exists {
		switch b.Kind {
		case varVertex:
			if len(b.Types) > 0 {
				return fmt.Sprintf("Vertex variable `%s` (Tag: %s)", b.Name, strings.Join(b.Types, ", ")), start, end, true
			}
			return fmt.Sprintf("Vertex variable `%s`", b.Name), start, end, true
		case varEdge:
			if len(b.Types) > 0 {
				return fmt.Sprintf("Edge variable `%s` (Edge: %s)", b.Name, strings.Join(b.Types, ", ")), start, end, true
			}
			return fmt.Sprintf("Edge variable `%s`", b.Name), start, end, true
		case varPath:
			return fmt.Sprintf("Path variable `%s`", b.Name), start, end, true
		case varAlias:
			return fmt.Sprintf("Column alias `%s`", b.Name), start, end, true
		}
	}

	// Check graph space names
	for _, sp := range spaces {
		if strings.EqualFold(sp, word) {
			return fmt.Sprintf("Graph Space `%s`", sp), start, end, true
		}
	}

	return "", 0, 0, false
}

// HoverDoc returns hover documentation for a word under the pointer, checking schema first.
func HoverDoc(word string, schema *model.SpaceSchema) (string, bool) {
	if word == "" {
		return "", false
	}
	if schema != nil {
		if t := findTag(schema, word); t != nil {
			var lines []string
			lines = append(lines, fmt.Sprintf("TAG `%s` (%d vertices)", t.Name, t.Count))
			for _, p := range t.Properties {
				idxBadge := ""
				if len(schema.FieldIndexes("TAG", t.Name, p.Field)) > 0 {
					idxBadge = " [indexed]"
				}
				lines = append(lines, fmt.Sprintf("  • %s: %s%s", p.Field, p.Type, idxBadge))
			}
			return strings.Join(lines, "\n"), true
		}
		if e := findEdge(schema, word); e != nil {
			var lines []string
			lines = append(lines, fmt.Sprintf("EDGE `%s` (%d edges)", e.Name, e.Count))
			for _, p := range e.Properties {
				idxBadge := ""
				if len(schema.FieldIndexes("EDGE", e.Name, p.Field)) > 0 {
					idxBadge = " [indexed]"
				}
				lines = append(lines, fmt.Sprintf("  • %s: %s%s", p.Field, p.Type, idxBadge))
			}
			return strings.Join(lines, "\n"), true
		}
	}
	if doc, ok := keywordDocs[strings.ToUpper(word)]; ok {
		return doc, true
	}
	if fn, ok := funcByLower[strings.ToLower(word)]; ok {
		return fn.signature + "\n" + fn.doc, true
	}
	return "", false
}
