package ngql

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui/editor"
)

const stateBlockComment uint64 = 1

var keywords = map[string]bool{
	"MATCH": true, "OPTIONAL": true, "GO": true, "FROM": true, "TO": true, "OVER": true,
	"WHERE": true, "YIELD": true, "RETURN": true, "LIMIT": true, "SKIP": true, "OFFSET": true,
	"ORDER": true, "BY": true, "ASC": true, "DESC": true, "AS": true, "AND": true, "OR": true,
	"XOR": true, "NOT": true, "IN": true, "CONTAINS": true, "STARTS": true, "ENDS": true,
	"WITH": true, "DISTINCT": true, "UNION": true, "ALL": true, "FETCH": true, "PROP": true,
	"PROPS": true, "ON": true, "LOOKUP": true, "GET": true, "SUBGRAPH": true, "STEP": true,
	"STEPS": true, "BOTH": true, "OUT": true, "REVERSELY": true, "BIDIRECT": true, "FIND": true,
	"SHORTEST": true, "NOLOOP": true, "PATH": true, "UPTO": true, "INSERT": true, "VERTEX": true,
	"VERTICES": true, "EDGE": true, "EDGES": true, "VALUES": true, "UPDATE": true, "UPSERT": true,
	"SET": true, "WHEN": true, "DELETE": true, "DETACH": true, "CREATE": true, "ALTER": true,
	"DROP": true, "ADD": true, "CHANGE": true, "SPACE": true, "SPACES": true, "TAG": true,
	"TAGS": true, "INDEX": true, "INDEXES": true, "STATUS": true, "SHOW": true, "STATS": true,
	"HOSTS": true, "PARTS": true, "JOBS": true, "SESSIONS": true, "QUERIES": true, "CHARSET": true,
	"COLLATION": true, "USERS": true, "ROLES": true, "META": true, "STORAGE": true, "GRAPH": true,
	"DESCRIBE": true, "USE": true, "EXPLAIN": true, "PROFILE": true, "SUBMIT": true, "JOB": true,
	"REBUILD": true, "COMPACT": true, "FLUSH": true, "BALANCE": true, "GROUP": true, "UNWIND": true,
	"CASE": true, "THEN": true, "ELSE": true, "END": true, "IS": true, "IF": true, "EXISTS": true,
	"TTL": true, "TTL_DURATION": true, "TTL_COL": true, "DEFAULT": true, "COMMENT": true,
	"VID_TYPE": true, "PARTITION_NUM": true, "REPLICA_FACTOR": true, "SAMPLE": true, "OF": true,
}

var constants = map[string]bool{
	"TRUE": true, "FALSE": true, "NULL": true, "EMPTY": true, "NAN": true, "INF": true,
}

var dataTypes = map[string]bool{
	"INT": true, "INT8": true, "INT16": true, "INT32": true, "INT64": true,
	"FLOAT": true, "DOUBLE": true, "STRING": true, "FIXED_STRING": true, "BOOL": true,
	"DATE": true, "TIME": true, "DATETIME": true, "TIMESTAMP": true, "DURATION": true,
	"GEOGRAPHY": true, "LIST": true, "MAP": true,
}

var builtins = map[string]bool{
	"id": true, "tags": true, "labels": true, "properties": true, "type": true,
	"src": true, "dst": true, "rank": true, "nodes": true, "relationships": true,
	"length": true, "count": true, "sum": true, "avg": true, "min": true, "max": true,
	"collect": true, "size": true, "keys": true, "head": true, "last": true, "tail": true,
	"range": true, "coalesce": true, "tolower": true, "toupper": true, "trim": true,
	"ltrim": true, "rtrim": true, "substring": true, "replace": true, "split": true,
	"now": true, "datetime": true, "date": true, "time": true, "duration": true,
	"abs": true, "round": true, "ceil": true, "floor": true, "sqrt": true, "pow": true,
	"rand": true, "tostring": true, "tointeger": true, "tofloat": true, "toboolean": true,
	"startnode": true, "endnode": true, "exists": true, "isempty": true, "udf_is_in": true,
}

// Language returns a custom mygo/ui/editor.Language for NebulaGraph nGQL.
func Language() *editor.Language {
	notInCode := []editor.TokenKind{editor.TokenString, editor.TokenComment}
	return &editor.Language{
		Name:         "nGQL",
		Extensions:   []string{".ngql", ".nql"},
		LineComment:  "//",
		BlockComment: [2]string{"/*", "*/"},
		TabSize:      2,
		Brackets: []editor.BracketPair{
			{Open: "(", Close: ")"},
			{Open: "[", Close: "]"},
			{Open: "{", Close: "}"},
		},
		AutoClose: []editor.BracketPair{
			{Open: "(", Close: ")", NotIn: notInCode},
			{Open: "[", Close: "]", NotIn: notInCode},
			{Open: "{", Close: "}", NotIn: notInCode},
			{Open: `"`, Close: `"`, NotIn: notInCode},
			{Open: "'", Close: "'", NotIn: notInCode},
			{Open: "`", Close: "`", NotIn: notInCode},
		},
		Tokenizer: editor.TokenizerFunc(tokenizeLine),
	}
}

func tokenizeLine(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	n := len(line)
	i := 0
	emit := func(end int, kind editor.TokenKind) {
		if end <= i {
			return
		}
		if len(tokens) > 0 && tokens[len(tokens)-1].Kind == kind {
			tokens[len(tokens)-1].End = end
		} else {
			tokens = append(tokens, editor.Token{End: end, Kind: kind})
		}
		i = end
	}

	if state == stateBlockComment {
		if idx := strings.Index(line, "*/"); idx >= 0 {
			emit(idx+2, editor.TokenComment)
			state = 0
		} else {
			emit(n, editor.TokenComment)
			return tokens, stateBlockComment
		}
	}

	for i < n {
		ch := line[i]

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\r' {
			j := i + 1
			for j < n && (line[j] == ' ' || line[j] == '\t' || line[j] == '\r') {
				j++
			}
			emit(j, editor.TokenText)
			continue
		}

		// Line comments: #, //, --
		if ch == '#' || (ch == '/' && i+1 < n && line[i+1] == '/') || (ch == '-' && i+1 < n && line[i+1] == '-') {
			emit(n, editor.TokenComment)
			break
		}

		// Block comment: /* ... */
		if ch == '/' && i+1 < n && line[i+1] == '*' {
			if idx := strings.Index(line[i+2:], "*/"); idx >= 0 {
				emit(i+2+idx+2, editor.TokenComment)
				continue
			}
			emit(n, editor.TokenComment)
			return tokens, stateBlockComment
		}

		// Quoted strings: "..." or '...'
		if ch == '"' || ch == '\'' {
			quote := ch
			j := i + 1
			for j < n {
				if line[j] == '\\' && j+1 < n {
					j += 2
					continue
				}
				if line[j] == quote {
					j++
					break
				}
				j++
			}
			emit(j, editor.TokenString)
			continue
		}

		// Backtick identifiers: `tag_name`
		if ch == '`' {
			j := i + 1
			for j < n && line[j] != '`' {
				j++
			}
			if j < n {
				j++
			}
			emit(j, editor.TokenProperty)
			continue
		}

		// NebulaGraph variables: $^, $$, $-, $param
		if ch == '$' {
			j := i + 1
			if j < n && (line[j] == '^' || line[j] == '$' || line[j] == '-') {
				j++
			} else {
				for j < n && isIdentPart(line[j]) {
					j++
				}
			}
			emit(j, editor.TokenVariable)
			continue
		}

		// Numbers
		if ch >= '0' && ch <= '9' {
			j := i + 1
			hasDot := false
			for j < n {
				c := line[j]
				if c >= '0' && c <= '9' {
					j++
				} else if c == '.' && !hasDot && j+1 < n && line[j+1] >= '0' && line[j+1] <= '9' {
					hasDot = true
					j++
				} else {
					break
				}
			}
			emit(j, editor.TokenNumber)
			continue
		}

		// Identifiers, keywords, types, functions
		if isIdentStart(ch) {
			j := i + 1
			for j < n && isIdentPart(line[j]) {
				j++
			}
			word := line[i:j]
			upper := strings.ToUpper(word)
			lower := strings.ToLower(word)

			// Lookahead for function call '('
			k := j
			for k < n && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			isCall := k < n && line[k] == '('

			switch {
			case constants[upper]:
				emit(j, editor.TokenConstant)
			case isCall && builtins[lower]:
				emit(j, editor.TokenFunction)
			case keywords[upper]:
				emit(j, editor.TokenKeyword)
			case dataTypes[upper]:
				emit(j, editor.TokenType)
			case isCall:
				emit(j, editor.TokenFunction)
			default:
				emit(j, editor.TokenText)
			}
			continue
		}

		// Multi-char operators: ->, <-, ==, !=, <=, >=, =~, .., ::
		if i+1 < n {
			two := line[i : i+2]
			if two == "->" || two == "<-" || two == "==" || two == "!=" ||
				two == "<=" || two == ">=" || two == "=~" || two == ".." || two == "::" {
				emit(i+2, editor.TokenOperator)
				continue
			}
		}

		// Operators & punctuation
		if strings.IndexByte("+-*/%=<>!|&~^@", ch) >= 0 {
			emit(i+1, editor.TokenOperator)
			continue
		}
		if strings.IndexByte("()[]{}:,.;", ch) >= 0 {
			emit(i+1, editor.TokenPunctuation)
			continue
		}

		// Advance one UTF-8 rune fallback
		_, size := utf8Rune(line[i:])
		emit(i+size, editor.TokenText)
	}

	return tokens, state
}

func isIdentStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isIdentPart(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}

func utf8Rune(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return unicode.ReplacementChar, 1
}
