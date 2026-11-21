package ngql

import (
	"strings"
)

type tokKind uint8

const (
	tokWord tokKind = iota
	tokString
	tokNumber
	tokVar      // $^, $$, $-, $param
	tokBacktick // `ident`
	tokPunct    // ( ) [ ] { } : . , ; | * -> <- -
	tokOp       // == != <= >= < > = + / % etc.
)

type stmtToken struct {
	kind  tokKind
	text  string // raw source slice
	value string // unquoted text for backticks/strings, else raw text
	upper string // uppercase value for words/backticks
	start int    // byte offset in full source
	end   int    // byte offset in full source
}

func (t stmtToken) isIdent() bool {
	return t.kind == tokWord || t.kind == tokBacktick
}

type varKind uint8

const (
	varVertex varKind = iota
	varEdge
	varPath
	varAlias
)

type varBinding struct {
	Name  string
	Kind  varKind
	Types []string // tag names for varVertex, edge types for varEdge
}

type delimFrame struct {
	open       byte     // '(', '[', '{'
	tokIdx     int      // index of opening token in pipeTokens
	isPattern  bool     // true for MATCH vertex (...) or edge [...] pattern
	ownerTypes []string // tags or edge types attached to this pattern (or enclosing pattern for '{')
	ownerIsEdg bool     // true if ownerTypes are edge types
}

type analyzedContext struct {
	inComment  bool
	inString   bool
	inBacktick bool
	prevChar   byte // byte immediately before wordStart (0 if none)

	wordStart int
	stmtStart int
	stmtEnd   int

	allTokens      []stmtToken // tokens of entire current statement
	beforeTokens   []stmtToken // tokens of current statement before wordStart
	pipeTokens     []stmtToken // tokens of active pipe segment before wordStart
	prevPipeTokens []stmtToken // tokens of previous pipe segment (for $-.)

	hasPipe     bool
	pipeAliases []string
	bindings    map[string]*varBinding
	bindingList []*varBinding
	overEdges   []string
	onTarget    string

	delimStack []delimFrame
	stmtHead   string // first keyword of active pipe segment (uppercase)
	clause     string // active clause keyword in active pipe segment
}

// analyzeContext lexes the active nGQL statement around offset and extracts
// cursor state, dot chains, enclosing pattern brackets, and bound variables.
func analyzeContext(src string, offset int, prefix string) *analyzedContext {
	if offset < 0 {
		offset = 0
	}
	if offset > len(src) {
		offset = len(src)
	}
	wordStart := offset - len(prefix)
	if wordStart < 0 {
		wordStart = 0
	}

	ac := &analyzedContext{
		wordStart: wordStart,
		bindings:  make(map[string]*varBinding),
	}

	// Scan from 0 to len(src) to accurately track comments, strings, semicolons, and statement boundaries.
	n := len(src)
	i := 0
	stmtStart := 0
	stmtEnd := n
	foundStmtEnd := false

	for i < n {
		// Check if offset falls inside a comment or string/backtick while scanning.
		ch := src[i]

		// Line comments: #, //, --
		if ch == '#' || (ch == '/' && i+1 < n && src[i+1] == '/') || (ch == '-' && i+1 < n && src[i+1] == '-') {
			end := i
			for end < n && src[end] != '\n' {
				end++
			}
			if offset > i && offset <= end {
				ac.inComment = true
				return ac
			}
			i = end
			continue
		}

		// Block comment: /* ... */
		if ch == '/' && i+1 < n && src[i+1] == '*' {
			closeIdx := strings.Index(src[i+2:], "*/")
			end := n
			if closeIdx >= 0 {
				end = i + 2 + closeIdx + 2
			}
			if offset >= i+2 && (closeIdx < 0 || offset <= end-2) {
				ac.inComment = true
				return ac
			}
			i = end
			continue
		}

		// Quoted string: "..." or '...'
		if ch == '"' || ch == '\'' {
			quote := ch
			j := i + 1
			closed := false
			for j < n {
				if src[j] == '\\' && j+1 < n {
					j += 2
					continue
				}
				if src[j] == quote {
					j++
					closed = true
					break
				}
				if src[j] == '\n' {
					break
				}
				j++
			}
			if offset > i && ((!closed && offset <= j) || (closed && offset < j)) {
				ac.inString = true
				// Keep wordStart inside string; stop scanning statement tokens before the opening quote.
				stmtEnd = i
				foundStmtEnd = true
				break
			}
			i = j
			continue
		}

		// Backtick identifier: `...`
		if ch == '`' {
			j := i + 1
			closed := false
			for j < n && src[j] != '`' && src[j] != '\n' {
				j++
			}
			if j < n && src[j] == '`' {
				j++
				closed = true
			}
			if offset > i && ((!closed && offset <= j) || (closed && offset < j)) {
				ac.inBacktick = true
				// Adjust wordStart to right after opening backtick if prefix started inside backtick.
				if wordStart <= i {
					wordStart = i + 1
					ac.wordStart = wordStart
				}
			}
			i = j
			continue
		}

		// Statement separator ';'
		if ch == ';' {
			if i < offset {
				stmtStart = i + 1
			} else if !foundStmtEnd {
				stmtEnd = i
				foundStmtEnd = true
				break
			}
			i++
			continue
		}

		i++
	}

	ac.stmtStart = stmtStart
	ac.stmtEnd = stmtEnd

	// Determine previous non-partial byte immediately before wordStart (if inside statement).
	if wordStart > stmtStart && wordStart <= len(src) {
		ac.prevChar = src[wordStart-1]
	}

	// Tokenize entire active statement for binding extraction.
	fullEnd := stmtEnd
	if ac.inString {
		// When inside a string, tokenize up to the opening quote.
		ac.allTokens = lexTokens(src, stmtStart, stmtEnd)
		ac.beforeTokens = ac.allTokens
	} else {
		ac.allTokens = lexTokens(src, stmtStart, fullEnd)
		// Tokens strictly before wordStart (or before backtick open if inBacktick).
		beforeCut := wordStart
		if ac.inBacktick && beforeCut > stmtStart && src[beforeCut-1] == '`' {
			beforeCut--
		}
		ac.beforeTokens = lexTokens(src, stmtStart, beforeCut)
	}

	// Split beforeTokens by top-level pipe '|' (bracket depth 0).
	ac.splitPipes()

	// Extract variable bindings & statement targets from full statement.
	ac.extractBindings()

	// Extract pipe aliases from previous pipe segment.
	ac.extractPipeAliases()

	// Build unmatched delimiter stack on active pipeTokens.
	ac.buildDelimStack()

	// Determine statement head & active clause in active pipe segment.
	ac.detectClause()

	return ac
}

// lexTokens tokenizes src[start:end], skipping whitespace and comments.
func lexTokens(src string, start, end int) []stmtToken {
	if start < 0 {
		start = 0
	}
	if end > len(src) {
		end = len(src)
	}
	if start >= end {
		return nil
	}
	var toks []stmtToken
	i := start
	for i < end {
		ch := src[i]

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			i++
			continue
		}

		// Line comments
		if ch == '#' || (ch == '/' && i+1 < end && src[i+1] == '/') || (ch == '-' && i+1 < end && src[i+1] == '-') {
			for i < end && src[i] != '\n' {
				i++
			}
			continue
		}

		// Block comments
		if ch == '/' && i+1 < end && src[i+1] == '*' {
			if idx := strings.Index(src[i+2:end], "*/"); idx >= 0 {
				i = i + 2 + idx + 2
			} else {
				break
			}
			continue
		}

		// Strings
		if ch == '"' || ch == '\'' {
			quote := ch
			j := i + 1
			for j < end {
				if src[j] == '\\' && j+1 < end {
					j += 2
					continue
				}
				if src[j] == quote {
					j++
					break
				}
				j++
			}
			raw := src[i:j]
			val := strings.Trim(raw, string(quote))
			toks = append(toks, stmtToken{kind: tokString, text: raw, value: val, start: i, end: j})
			i = j
			continue
		}

		// Backtick identifiers
		if ch == '`' {
			j := i + 1
			for j < end && src[j] != '`' && src[j] != '\n' {
				j++
			}
			val := src[i+1 : j]
			if j < end && src[j] == '`' {
				j++
			}
			toks = append(toks, stmtToken{
				kind:  tokBacktick,
				text:  src[i:j],
				value: val,
				upper: strings.ToUpper(val),
				start: i,
				end:   j,
			})
			i = j
			continue
		}

		// NebulaGraph variables: $^, $$, $-, $param
		if ch == '$' {
			j := i + 1
			if j < end && (src[j] == '^' || src[j] == '$' || src[j] == '-') {
				j++
			} else {
				for j < end && isIdentPart(src[j]) {
					j++
				}
			}
			raw := src[i:j]
			toks = append(toks, stmtToken{kind: tokVar, text: raw, value: raw, upper: strings.ToUpper(raw), start: i, end: j})
			i = j
			continue
		}

		// Numbers
		if ch >= '0' && ch <= '9' {
			j := i + 1
			for j < end && ((src[j] >= '0' && src[j] <= '9') || (src[j] == '.' && j+1 < end && src[j+1] >= '0' && src[j+1] <= '9')) {
				j++
			}
			raw := src[i:j]
			toks = append(toks, stmtToken{kind: tokNumber, text: raw, value: raw, start: i, end: j})
			i = j
			continue
		}

		// Words / Identifiers
		if isIdentStart(ch) {
			j := i + 1
			for j < end && isIdentPart(src[j]) {
				j++
			}
			raw := src[i:j]
			toks = append(toks, stmtToken{kind: tokWord, text: raw, value: raw, upper: strings.ToUpper(raw), start: i, end: j})
			i = j
			continue
		}

		// Multi-char operators & arrows
		if i+1 < end {
			two := src[i : i+2]
			if two == "->" || two == "<-" || two == ".." || two == "::" {
				toks = append(toks, stmtToken{kind: tokPunct, text: two, value: two, start: i, end: i + 2})
				i += 2
				continue
			}
			if two == "==" || two == "!=" || two == "<=" || two == ">=" || two == "=~" {
				toks = append(toks, stmtToken{kind: tokOp, text: two, value: two, start: i, end: i + 2})
				i += 2
				continue
			}
		}

		// Single-char punctuation
		if strings.IndexByte("()[]{}:.,;|*-", ch) >= 0 {
			raw := src[i : i+1]
			toks = append(toks, stmtToken{kind: tokPunct, text: raw, value: raw, start: i, end: i + 1})
			i++
			continue
		}

		// Single-char operator
		if strings.IndexByte("=<>!+/%&~^@", ch) >= 0 {
			raw := src[i : i+1]
			toks = append(toks, stmtToken{kind: tokOp, text: raw, value: raw, start: i, end: i + 1})
			i++
			continue
		}

		// Skip any other UTF-8 rune
		_, size := utf8Rune(src[i:end])
		i += size
	}
	return toks
}

func (ac *analyzedContext) splitPipes() {
	depthParen, depthBrack, depthBrace := 0, 0, 0
	var segments [][]stmtToken
	segStart := 0
	for i, tk := range ac.beforeTokens {
		if tk.kind == tokPunct {
			switch tk.text {
			case "(":
				depthParen++
			case ")":
				if depthParen > 0 {
					depthParen--
				}
			case "[":
				depthBrack++
			case "]":
				if depthBrack > 0 {
					depthBrack--
				}
			case "{":
				depthBrace++
			case "}":
				if depthBrace > 0 {
					depthBrace--
				}
			case "|":
				if depthParen == 0 && depthBrack == 0 && depthBrace == 0 {
					segments = append(segments, ac.beforeTokens[segStart:i])
					segStart = i + 1
				}
			}
		}
	}
	segments = append(segments, ac.beforeTokens[segStart:])
	if len(segments) >= 2 {
		ac.hasPipe = true
		ac.prevPipeTokens = segments[len(segments)-2]
	}
	ac.pipeTokens = segments[len(segments)-1]
}

func (ac *analyzedContext) addBinding(name string, kind varKind, types []string) {
	if name == "" || keywords[strings.ToUpper(name)] {
		return
	}
	if existing, ok := ac.bindings[name]; ok {
		// Merge types if newly discovered
		if len(types) > 0 {
			seen := make(map[string]bool, len(existing.Types))
			for _, t := range existing.Types {
				seen[t] = true
			}
			for _, t := range types {
				if !seen[t] {
					existing.Types = append(existing.Types, t)
					seen[t] = true
				}
			}
		}
		return
	}
	b := &varBinding{Name: name, Kind: kind, Types: append([]string(nil), types...)}
	ac.bindings[name] = b
	ac.bindingList = append(ac.bindingList, b)
}

func (ac *analyzedContext) extractBindings() {
	toks := ac.allTokens
	n := len(toks)
	for i := 0; i < n; i++ {
		tk := toks[i]

		// Path variable: p = (...)
		if tk.isIdent() && i+2 < n && toks[i+1].text == "=" && toks[i+2].text == "(" {
			ac.addBinding(tk.value, varPath, nil)
			continue
		}

		// Alias: AS ident
		if tk.kind == tokWord && tk.upper == "AS" && i+1 < n && toks[i+1].isIdent() {
			ac.addBinding(toks[i+1].value, varAlias, nil)
			continue
		}

		// OVER edge1, edge2
		if tk.kind == tokWord && tk.upper == "OVER" {
			j := i + 1
			for j < n {
				if toks[j].isIdent() && !keywords[toks[j].upper] {
					ac.overEdges = appendUnique(ac.overEdges, toks[j].value)
					j++
					if j < n && toks[j].text == "," {
						j++
						continue
					}
				}
				break
			}
			continue
		}

		// LOOKUP ON target / FETCH PROP ON target
		if tk.kind == tokWord && tk.upper == "ON" && i > 0 {
			prev := toks[i-1]
			if prev.kind == tokWord && (prev.upper == "LOOKUP" || prev.upper == "PROP" || prev.upper == "PROPS") {
				if i+1 < n && toks[i+1].isIdent() && !keywords[toks[i+1].upper] {
					ac.onTarget = toks[i+1].value
				}
			}
			continue
		}

		// Vertex pattern: (var:tag1:tag2 { ... })
		if tk.kind == tokPunct && tk.text == "(" {
			if !isNodePatternOpen(toks, i) {
				continue
			}
			j := i + 1
			var varName string
			var tags []string
			if j < n && toks[j].isIdent() && !keywords[toks[j].upper] {
				// Confirm next token is ), :, {, or end (so `(a + b)` isn't misread as vertex var)
				if j+1 >= n || toks[j+1].text == ")" || toks[j+1].text == ":" || toks[j+1].text == "{" {
					varName = toks[j].value
					j++
				}
			}
			for j < n && toks[j].text == ":" {
				j++
				if j < n && toks[j].isIdent() {
					tags = append(tags, toks[j].value)
					j++
				}
			}
			if varName != "" {
				ac.addBinding(varName, varVertex, tags)
			}
			continue
		}

		// Edge pattern: [e:follow|serve*1..3]
		if tk.kind == tokPunct && tk.text == "[" {
			if !isEdgePatternOpen(toks, i) {
				continue
			}
			j := i + 1
			var varName string
			var edges []string
			if j < n && toks[j].isIdent() && !keywords[toks[j].upper] {
				if j+1 >= n || toks[j+1].text == "]" || toks[j+1].text == ":" || toks[j+1].text == "*" || toks[j+1].text == "{" {
					varName = toks[j].value
					j++
				}
			}
			if j < n && toks[j].text == ":" {
				j++
				for j < n {
					if toks[j].isIdent() {
						edges = append(edges, toks[j].value)
						j++
					}
					if j < n && (toks[j].text == "|" || toks[j].text == ":") {
						j++
						continue
					}
					break
				}
			}
			if varName != "" {
				ac.addBinding(varName, varEdge, edges)
			}
			continue
		}
	}
}

func (ac *analyzedContext) extractPipeAliases() {
	toks := ac.prevPipeTokens
	if len(toks) == 0 {
		return
	}
	// Scan for YIELD / RETURN clause in previous pipe segment
	yieldIdx := -1
	for i, tk := range toks {
		if tk.kind == tokWord && (tk.upper == "YIELD" || tk.upper == "RETURN") {
			yieldIdx = i
		}
	}
	if yieldIdx < 0 {
		return
	}
	for i := yieldIdx + 1; i < len(toks); i++ {
		if toks[i].kind == tokWord && toks[i].upper == "AS" && i+1 < len(toks) && toks[i+1].isIdent() {
			ac.pipeAliases = appendUnique(ac.pipeAliases, toks[i+1].value)
		}
	}
}

func (ac *analyzedContext) buildDelimStack() {
	toks := ac.pipeTokens
	var stack []delimFrame
	for i, tk := range toks {
		if tk.kind != tokPunct {
			continue
		}
		switch tk.text {
		case "(":
			isPat := isNodePatternOpen(toks, i)
			var types []string
			if isPat {
				types = collectPatternTypes(toks, i, false)
			}
			stack = append(stack, delimFrame{open: '(', tokIdx: i, isPattern: isPat, ownerTypes: types})
		case "[":
			isPat := isEdgePatternOpen(toks, i)
			var types []string
			if isPat {
				types = collectPatternTypes(toks, i, true)
			}
			stack = append(stack, delimFrame{open: '[', tokIdx: i, isPattern: isPat, ownerTypes: types, ownerIsEdg: true})
		case "{":
			frame := delimFrame{open: '{', tokIdx: i}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if parent.isPattern {
					frame.isPattern = true
					frame.ownerTypes = parent.ownerTypes
					frame.ownerIsEdg = parent.ownerIsEdg
				}
			}
			stack = append(stack, frame)
		case ")":
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k].open == '(' {
					stack = stack[:k]
					break
				}
			}
		case "]":
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k].open == '[' {
					stack = stack[:k]
					break
				}
			}
		case "}":
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k].open == '{' {
					stack = stack[:k]
					break
				}
			}
		}
	}
	ac.delimStack = stack
}

func (ac *analyzedContext) detectClause() {
	toks := ac.pipeTokens
	if len(toks) == 0 {
		ac.clause = "START"
		return
	}

	startIdx := 0
	// Skip leading EXPLAIN / PROFILE when finding statement head
	if toks[0].kind == tokWord && (toks[0].upper == "EXPLAIN" || toks[0].upper == "PROFILE") {
		if len(toks) == 1 {
			ac.stmtHead = toks[0].upper
			ac.clause = toks[0].upper
			return
		}
		startIdx = 1
		if toks[1].kind == tokWord && toks[1].upper == "FORMAT" && len(toks) > 3 {
			startIdx = 3
		}
	}
	if startIdx < len(toks) && toks[startIdx].kind == tokWord {
		ac.stmtHead = toks[startIdx].upper
	}

	depth := 0
	clause := ac.stmtHead
	for i := startIdx; i < len(toks); i++ {
		tk := toks[i]
		if tk.kind == tokPunct {
			switch tk.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				if depth > 0 {
					depth--
				}
			}
			continue
		}
		if depth != 0 || tk.kind != tokWord {
			continue
		}
		switch tk.upper {
		case "MATCH", "WHERE", "RETURN", "YIELD", "WITH", "LIMIT", "SKIP", "OVER", "FROM", "TO", "SET", "WHEN", "VALUES", "UNWIND":
			clause = tk.upper
		case "ORDER":
			if i+1 < len(toks) && toks[i+1].upper == "BY" {
				clause = "ORDER_BY"
			} else {
				clause = "ORDER"
			}
		case "GROUP":
			if i+1 < len(toks) && toks[i+1].upper == "BY" {
				clause = "GROUP_BY"
			} else {
				clause = "GROUP"
			}
		}
	}
	ac.clause = clause
}

// isNodePatternOpen checks if '(' at idx in toks opens a MATCH vertex pattern (not a function call or WHERE grouping).
func isNodePatternOpen(toks []stmtToken, idx int) bool {
	if idx == 0 {
		return false
	}
	prev := toks[idx-1]
	// Right after -, ->, <-, = (path p=()), or MATCH / OPTIONAL MATCH
	if prev.kind == tokPunct && (prev.text == "-" || prev.text == "->" || prev.text == "<-") {
		return true
	}
	if prev.kind == tokOp && prev.text == "=" {
		return true
	}
	if prev.kind == tokWord && prev.upper == "MATCH" {
		return true
	}
	// Comma inside MATCH pattern: MATCH (a), (b)
	if prev.kind == tokPunct && prev.text == "," {
		// Walk backward at depth 0 to see if we're in MATCH clause
		depth := 0
		for k := idx - 1; k >= 0; k-- {
			t := toks[k]
			if t.kind == tokPunct {
				switch t.text {
				case ")", "]", "}":
					depth++
				case "(", "[", "{":
					if depth > 0 {
						depth--
					} else {
						return false
					}
				}
			}
			if depth == 0 && t.kind == tokWord {
				switch t.upper {
				case "MATCH":
					return true
				case "WHERE", "RETURN", "YIELD", "WITH", "ORDER", "GROUP", "CREATE", "INSERT", "VALUES", "SET":
					return false
				}
			}
		}
	}
	return false
}

// isEdgePatternOpen checks if '[' at idx in toks opens a MATCH edge pattern (e.g. -[e:follow]-> or <-[e]-).
func isEdgePatternOpen(toks []stmtToken, idx int) bool {
	if idx == 0 {
		return false
	}
	prev := toks[idx-1]
	if prev.kind == tokPunct && (prev.text == "-" || prev.text == "<-") {
		return true
	}
	return false
}

// collectPatternTypes extracts tag or edge names after ':' inside an open '(' or '[' pattern.
func collectPatternTypes(toks []stmtToken, openIdx int, isEdge bool) []string {
	var out []string
	j := openIdx + 1
	// Skip optional variable name
	if j < len(toks) && toks[j].isIdent() {
		j++
	}
	for j < len(toks) {
		tk := toks[j]
		if tk.kind == tokPunct && (tk.text == ")" || tk.text == "]" || tk.text == "{" || tk.text == "*") {
			break
		}
		if tk.kind == tokPunct && (tk.text == ":" || (isEdge && tk.text == "|")) {
			j++
			if j < len(toks) && toks[j].isIdent() {
				out = appendUnique(out, toks[j].value)
				j++
			}
			continue
		}
		j++
	}
	return out
}

// trailingDotChain returns the dot-separated qualifier segments right before the trailing '.',
// e.g. for `v.player.` returns `["v", "player"], true`; for `$^.` returns `["$^"], true`.
func (ac *analyzedContext) trailingDotChain() ([]string, bool) {
	toks := ac.pipeTokens
	if len(toks) < 2 {
		return nil, false
	}
	last := toks[len(toks)-1]
	if last.kind != tokPunct || last.text != "." {
		return nil, false
	}
	var chain []string
	idx := len(toks) - 2
	for idx >= 0 {
		tk := toks[idx]
		if tk.isIdent() || tk.kind == tokVar {
			chain = append([]string{tk.value}, chain...)
			idx--
			if idx >= 0 && toks[idx].kind == tokPunct && toks[idx].text == "." {
				idx--
				continue
			}
			break
		}
		break
	}
	if len(chain) == 0 {
		return nil, false
	}
	return chain, true
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
