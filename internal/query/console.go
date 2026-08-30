package query

import (
	"context"
	"database/sql"
	"fmt"

	"datapeek/internal/config"
)

// MaxConsoleRows caps console query results.
const MaxConsoleRows = 1000

var readOnlyVerbs = map[string]bool{
	"select": true, "show": true, "explain": true, "describe": true,
	"desc": true, "with": true, "table": true, "values": true,
}

var dmlVerbs = map[string]bool{
	"insert": true, "update": true, "delete": true, "merge": true,
	"truncate": true, "replace": true,
}

// ClassifyStatement validates that sqlText is a single statement and
// reports its leading verb and whether it is read-only for dbType.
// The allowlist is strict: anything that is not clearly read-only is
// treated as a write.
func ClassifyStatement(dbType config.DBType, sqlText string) (verb string, readOnly bool, err error) {
	stmts, err := splitStatements(sqlText)
	if err != nil {
		return "", false, err
	}
	if len(stmts) == 0 {
		return "", false, fmt.Errorf("empty statement")
	}
	stmt := stmts[0]
	toks := tokens(stmt)
	if len(toks) == 0 {
		return "", false, fmt.Errorf("empty statement")
	}
	verb = toks[0]
	switch {
	case readOnlyVerbs[verb]:
		readOnly = true
		// PG data-modifying CTEs: WITH x AS (DELETE ...) SELECT ...
		if verb == "with" && containsAny(toks, dmlVerbs) {
			readOnly = false
		}
		// EXPLAIN ANALYZE UPDATE … executes the modification.
		if verb == "explain" {
			j := 1
			for j < len(toks) && (toks[j] == "analyze" || toks[j] == "verbose") {
				j++
			}
			if j < len(toks) && dmlVerbs[toks[j]] {
				readOnly = false
			}
		}
		// PG SELECT INTO creates a table.
		if dbType == config.PostgreSQL && containsToken(toks, "into") {
			readOnly = false
		}
		// SELECT ... OUT FILE/DUMPFILE writes files (MySQL).
		if containsToken(toks, "outfile") || containsToken(toks, "dumpfile") {
			readOnly = false
		}
	case dmlVerbs[verb]:
		// write
	default:
		// DDL, SET, LOCK, GRANT, CALL, … all treated as writes
	}
	return verb, readOnly, nil
}

// RunConsoleQuery executes one validated statement. Read-only statements
// run as queries capped at MaxConsoleRows; anything else requires
// allowWrites and runs via Exec.
func RunConsoleQuery(ctx context.Context, db *sql.DB, dbType config.DBType, sqlText string, allowWrites bool) (*QueryResult, error) {
	verb, readOnly, err := ClassifyStatement(dbType, sqlText)
	if err != nil {
		return nil, err
	}
	if !readOnly && !allowWrites {
		return nil, fmt.Errorf("blocked: %q is not a read-only statement; enable writes to run it", verb)
	}
	if readOnly {
		return runConsoleRead(ctx, db, dbType, sqlText)
	}
	return runConsoleWrite(ctx, db, sqlText, verb)
}

func runConsoleRead(ctx context.Context, db *sql.DB, dbType config.DBType, sqlText string) (*QueryResult, error) {
	stmts, _ := splitStatements(sqlText)
	stmt := stmts[0]

	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	ct, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}
	var columns []ColumnMeta
	for _, c := range ct {
		columns = append(columns, ColumnMeta{
			Name:     c.Name(),
			DataType: columnType(dbType, c),
			Nullable: nullable(c),
		})
	}

	raw := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	result := &QueryResult{Columns: columns, Total: 0}
	for rows.Next() {
		if len(result.Rows) >= MaxConsoleRows {
			result.Truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		serialized := make([]any, len(raw))
		for i, v := range raw {
			serialized[i] = SerializeValue(v)
		}
		result.Rows = append(result.Rows, serialized)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.Total = int64(len(result.Rows))
	return result, nil
}

func runConsoleWrite(ctx context.Context, db *sql.DB, sqlText string, verb string) (*QueryResult, error) {
	stmts, _ := splitStatements(sqlText)
	res, err := db.ExecContext(ctx, stmts[0])
	if err != nil {
		return nil, fmt.Errorf("execute %s: %w", verb, err)
	}
	affected, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()
	return &QueryResult{
		Columns: []ColumnMeta{
			{Name: "status", DataType: "text"},
			{Name: "rows_affected", DataType: "bigint"},
			{Name: "last_insert_id", DataType: "bigint"},
		},
		Rows: [][]any{{fmt.Sprintf("OK (%s)", verb), affected, lastID}},
		Total: 1,
	}, nil
}

// splitStatements splits sqlText on top-level semicolons, ignoring ones
// inside string literals, quoted identifiers, and comments. Returns an
// error for unterminated constructs or multiple statements.
func splitStatements(sqlText string) ([]string, error) {
	var stmts []string
	var cur []byte
	n := len(sqlText)
	flush := func() {
		s := trimSpaces(string(cur))
		if s != "" {
			stmts = append(stmts, s)
		}
		cur = cur[:0]
	}
	for i := 0; i < n; {
		c := sqlText[i]
		switch {
		case c == '\'':
			j, ok := scanQuoted(sqlText, i, '\'')
			if !ok {
				return nil, fmt.Errorf("unterminated string literal")
			}
			cur = append(cur, sqlText[i:j]...)
			i = j
		case c == '"':
			j, ok := scanQuoted(sqlText, i, '"')
			if !ok {
				return nil, fmt.Errorf("unterminated quoted identifier")
			}
			cur = append(cur, sqlText[i:j]...)
			i = j
		case c == '`':
			j, ok := scanQuoted(sqlText, i, '`')
			if !ok {
				return nil, fmt.Errorf("unterminated quoted identifier")
			}
			cur = append(cur, sqlText[i:j]...)
			i = j
		case c == '-' && i+1 < n && sqlText[i+1] == '-':
			for i < n && sqlText[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && sqlText[i+1] == '*':
			j := i + 2
			for j+1 < n && !(sqlText[j] == '*' && sqlText[j+1] == '/') {
				j++
			}
			if j+1 >= n {
				return nil, fmt.Errorf("unterminated comment")
			}
			i = j + 2
		case c == ';':
			flush()
			i++
		default:
			cur = append(cur, c)
			i++
		}
	}
	flush()
	if len(stmts) > 1 {
		return nil, fmt.Errorf("multiple statements are not allowed")
	}
	return stmts, nil
}

// scanQuoted returns the index just past the closing quote, handling
// doubled-quote escapes ('' "" ``).
func scanQuoted(s string, start int, quote byte) (int, bool) {
	j := start + 1
	n := len(s)
	for j < n {
		if s[j] == quote {
			if j+1 < n && s[j+1] == quote {
				j += 2
				continue
			}
			return j + 1, true
		}
		j++
	}
	return 0, false
}

// tokens returns lowercased word tokens, skipping quoted sections.
func tokens(s string) []string {
	var out []string
	var w []byte
	n := len(s)
	for i := 0; i < n; {
		c := s[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			j, ok := scanQuoted(s, i, c)
			if !ok {
				j = n
			}
			i = j
			w = nil
		case isWordByte(c):
			w = append(w, c)
			i++
		default:
			if len(w) > 0 {
				out = append(out, string(w))
				w = nil
			}
			i++
		}
	}
	if len(w) > 0 {
		out = append(out, string(w))
	}
	for i, t := range out {
		out[i] = toLower(t)
	}
	return out
}

func isWordByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func toLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func containsAny(toks []string, set map[string]bool) bool {
	for _, t := range toks {
		if set[t] {
			return true
		}
	}
	return false
}

func containsToken(toks []string, token string) bool {
	for _, t := range toks {
		if t == token {
			return true
		}
	}
	return false
}

func trimSpaces(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}