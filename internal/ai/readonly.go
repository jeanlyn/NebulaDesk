package ai

import (
	"fmt"
	"strings"
)

var allowedReadOnlyStarters = map[string]bool{
	"MATCH":    true,
	"OPTIONAL": true,
	"GO":       true,
	"GET":      true,
	"FIND":     true,
	"FETCH":    true,
	"LOOKUP":   true,
	"SHOW":     true,
	"DESCRIBE": true,
	"DESC":     true,
	"EXPLAIN":  true,
	"PROFILE":  true,
	"YIELD":    true,
	"RETURN":   true,
	"ORDER":    true,
	"GROUP":    true,
	"LIMIT":    true,
	"SKIP":     true,
	"UNWIND":   true,
	"WITH":     true,
}

var forbiddenWriteStarters = map[string]bool{
	"INSERT":  true,
	"UPDATE":  true,
	"UPSERT":  true,
	"DELETE":  true,
	"DETACH":  true,
	"DROP":    true,
	"CREATE":  true,
	"ALTER":   true,
	"CLEAR":   true,
	"SUBMIT":  true,
	"REBUILD": true,
	"KILL":    true,
	"GRANT":   true,
	"REVOKE":  true,
	"SIGN":    true,
	"ADD":     true,
	"REMOVE":  true,
	"USE":     true,
	"BALANCE": true,
	"STOP":    true,
}

// ValidateReadOnlyNGQL verifies that every statement and piped segment in query is strictly read-only.
// It rejects DDL/DML mutations (INSERT/UPDATE/DELETE/DROP/CREATE/ALTER/USE/SUBMIT/etc.) even when
// chained with ';' or piped with '|'.
func ValidateReadOnlyNGQL(query string) error {
	cleaned := stripCommentsAndStrings(query)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return fmt.Errorf("empty nGQL query")
	}

	stmts := splitTopLevel(cleaned, ';')
	checked := 0
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		pipes := splitTopLevel(stmt, '|')
		for _, seg := range pipes {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				continue
			}
			first := firstKeyword(seg)
			if first == "" {
				continue
			}
			checked++
			if forbiddenWriteStarters[first] {
				return fmt.Errorf("read-only mode blocks statement starting with %q", first)
			}
			if !allowedReadOnlyStarters[first] {
				return fmt.Errorf("statement starting with %q is not permitted in read-only exploration mode", first)
			}
			// Guard against EXPLAIN / PROFILE wrapping a mutation, e.g. EXPLAIN DELETE VERTEX ...
			if first == "EXPLAIN" || first == "PROFILE" {
				rest := strings.TrimSpace(seg[len(first):])
				if next := firstKeyword(rest); next != "" {
					if next == "FORMAT" {
						// Skip FORMAT="dot" style clause
						parts := strings.Fields(rest)
						if len(parts) >= 3 {
							next = firstKeyword(strings.Join(parts[2:], " "))
						}
					}
					if forbiddenWriteStarters[next] {
						return fmt.Errorf("read-only mode blocks %s wrapping %q", first, next)
					}
				}
			}
		}
	}
	if checked == 0 {
		return fmt.Errorf("empty nGQL query")
	}
	return nil
}

// stripCommentsAndStrings removes comments and replaces quoted string contents with spaces
// so semicolons, pipes, or keywords inside strings/comments cannot confuse validation.
func stripCommentsAndStrings(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	n := len(src)
	for i := 0; i < n; {
		ch := src[i]
		// Line comments: //, #, --
		if ch == '#' || (ch == '/' && i+1 < n && src[i+1] == '/') || (ch == '-' && i+1 < n && src[i+1] == '-') {
			for i < n && src[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
			continue
		}
		// Block comment: /* ... */
		if ch == '/' && i+1 < n && src[i+1] == '*' {
			if idx := strings.Index(src[i+2:], "*/"); idx >= 0 {
				i = i + 2 + idx + 2
			} else {
				break
			}
			b.WriteByte(' ')
			continue
		}
		// Strings: "...", '...'
		if ch == '"' || ch == '\'' {
			quote := ch
			i++
			for i < n {
				if src[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if src[i] == quote {
					i++
					break
				}
				i++
			}
			b.WriteString(`""`)
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

// splitTopLevel splits s by sep only at bracket depth 0 (ignoring () [] {}).
func splitTopLevel(s string, sep byte) []string {
	var out []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if s[i] == sep && depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}

func firstKeyword(seg string) string {
	seg = strings.TrimSpace(seg)
	i := 0
	for i < len(seg) {
		c := seg[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			i++
		} else {
			break
		}
	}
	if i == 0 {
		return ""
	}
	return strings.ToUpper(seg[:i])
}
