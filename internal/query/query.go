// Package query provides paginated, sorted table reads with type-aware
// value serialization for the datagrid.
package query

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"datapeek/internal/config"
)

// ColumnMeta describes one result column.
type ColumnMeta struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
	Nullable bool   `json:"nullable"`
}

// QueryResult is the wire contract for the datagrid.
// Values are JSON-ready: strings, numbers, booleans, or null.
type QueryResult struct {
	Columns []ColumnMeta `json:"columns"`
	Rows    [][]any      `json:"rows"`
	Total   int64        `json:"total"`
}

// SortSpec is a client-requested sort.
type SortSpec struct {
	Column string `json:"column"`
	Desc   bool   `json:"desc"`
}

// Limits guard against unbounded reads.
const (
	MaxPageSize     = 500
	DefaultPageSize = 100
	MaxRowOffset    = 1_000_000_000
)

// QuoteIdentifier quotes a single identifier for the given dialect.
func QuoteIdentifier(dbType config.DBType, ident string) (string, error) {
	if ident == "" || strings.ContainsAny(ident, "\x00") {
		return "", fmt.Errorf("invalid identifier %q", ident)
	}
	switch dbType {
	case config.MySQL:
		return "`" + strings.ReplaceAll(ident, "`", "``") + "`", nil
	case config.PostgreSQL:
		return "\"" + strings.ReplaceAll(ident, "\"", "\"\"") + "\"", nil
	default:
		return "", fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// QuoteQualifiedName quotes schema.table for the given dialect.
func QuoteQualifiedName(dbType config.DBType, schema, table string) (string, error) {
	q, err := QuoteIdentifier(dbType, table)
	if err != nil {
		return "", err
	}
	if schema != "" {
		sq, err := QuoteIdentifier(dbType, schema)
		if err != nil {
			return "", err
		}
		return sq + "." + q, nil
	}
	return q, nil
}

// clampPagination validates page (0-based) and pageSize.
func clampPagination(page, pageSize int) (int, int, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	if page < 0 {
		page = 0
	}
	offset := page * pageSize
	if offset > MaxRowOffset {
		return 0, 0, fmt.Errorf("offset %d exceeds limit", offset)
	}
	return offset, pageSize, nil
}

// ReadTable returns one page of rows from schema.table, optionally sorted,
// plus the total row count for pagination.
func ReadTable(ctx context.Context, db *sql.DB, dbType config.DBType, schema, table string, page, pageSize int, sort *SortSpec) (*QueryResult, error) {
	offset, pageSize, err := clampPagination(page, pageSize)
	if err != nil {
		return nil, err
	}

	qualified, err := QuoteQualifiedName(dbType, schema, table)
	if err != nil {
		return nil, err
	}

	// Discover columns (also validates the table exists).
	colQuery := fmt.Sprintf("SELECT * FROM %s LIMIT 0", qualified)
	colRows, err := db.QueryContext(ctx, colQuery)
	if err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}
	ct, err := colRows.ColumnTypes()
	colRows.Close()
	if err != nil {
		return nil, fmt.Errorf("read columns: %w", err)
	}
	var columns []ColumnMeta
	for _, c := range ct {
		columns = append(columns, ColumnMeta{
			Name:     c.Name(),
			DataType: columnType(dbType, c),
			Nullable: nullable(c),
		})
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s has no columns", qualified)
	}

	// Build ORDER BY from a validated column.
	orderBy := ""
	if sort != nil && sort.Column != "" {
		sc, err := QuoteIdentifier(dbType, sort.Column)
		if err != nil {
			return nil, err
		}
		if !containsColumn(columns, sort.Column) {
			return nil, fmt.Errorf("unknown sort column %q", sort.Column)
		}
		dir := "ASC"
		if sort.Desc {
			dir = "DESC"
		}
		orderBy = " ORDER BY " + sc + " " + dir
	}

	// Total count for the pagination footer.
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", qualified)
	if err := db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, fmt.Errorf("count rows: %w", err)
	}

	// Fetch the page.
	dataQuery := fmt.Sprintf("SELECT * FROM %s%s LIMIT %d OFFSET %d", qualified, orderBy, pageSize, offset)
	rows, err := db.QueryContext(ctx, dataQuery)
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}
	defer rows.Close()

	result := &QueryResult{Columns: columns, Total: total}
	raw := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		serialized := make([]any, len(raw))
		for i, v := range raw {
			serialized[i] = SerializeValue(v)
		}
		result.Rows = append(result.Rows, serialized)
	}
	return result, rows.Err()
}

func containsColumn(columns []ColumnMeta, name string) bool {
	for _, c := range columns {
		if c.Name == name {
			return true
		}
	}
	return false
}

func nullable(ct *sql.ColumnType) bool {
	n, ok := ct.Nullable()
	return !ok || n // unknown → assume nullable
}

func columnType(dbType config.DBType, ct *sql.ColumnType) string {
	if t := ct.DatabaseTypeName(); t != "" {
		if dbType == config.PostgreSQL && strings.EqualFold(t, "NUMERIC") {
			return t
		}
		return strings.ToLower(t)
	}
	return "unknown"
}

// SerializeValue converts a driver value into a JSON-friendly one.
// Time → RFC3339, valid UTF-8 bytes → string, binary → 0x-hex,
// NULL stays nil.
func SerializeValue(v any) any {
	switch val := v.(type) {
	case nil:
		return nil
	case time.Time:
		return val.UTC().Format(time.RFC3339Nano)
	case []byte:
		if utf8.Valid(val) {
			return string(val)
		}
		return "0x" + hex.EncodeToString(val)
	case int64:
		return val
	case float64:
		return formatFloat(val)
	case bool:
		return val
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// formatFloat avoids scientific notation for common values.
func formatFloat(f float64) any {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if n, err := strconv.ParseFloat(s, 64); err == nil && n == f {
		return s
	}
	return f
}
