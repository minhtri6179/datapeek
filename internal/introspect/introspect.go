// Package introspect walks database schemas (databases/schemas →
// tables/views → columns) using information_schema.
package introspect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"datapeek/internal/config"
)

// TableInfo describes a table or view.
type TableInfo struct {
	Name string `json:"name"`
	Type string `json:"type"` // "table" | "view"
}

// ColumnInfo describes a column.
type ColumnInfo struct {
	Name         string `json:"name"`
	DataType     string `json:"dataType"`
	Nullable     bool   `json:"nullable"`
	IsPrimaryKey bool   `json:"isPrimaryKey"`
}

// Schemas lists user schemas (PG) or databases (MySQL) visible to the user.
func Schemas(ctx context.Context, db *sql.DB, dbType config.DBType) ([]string, error) {
	var query string
	switch dbType {
	case config.MySQL:
		query = `SELECT schema_name FROM information_schema.schemata
			WHERE schema_name NOT IN ('mysql','information_schema','performance_schema','sys')
			ORDER BY schema_name`
	case config.PostgreSQL:
		query = `SELECT schema_name FROM information_schema.schemata
			WHERE schema_name NOT IN ('pg_catalog','information_schema')
			AND schema_name NOT LIKE 'pg_toast%'
			ORDER BY schema_name`
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Tables lists tables and views in a schema.
func Tables(ctx context.Context, db *sql.DB, dbType config.DBType, schema string) ([]TableInfo, error) {
	if schema == "" {
		return nil, fmt.Errorf("schema is required")
	}
	const pgQuery = `SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = $1
		ORDER BY table_name`
	const mysqlQuery = `SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = ?
		ORDER BY table_name`
	query := pgQuery
	if dbType == config.MySQL {
		query = mysqlQuery
	}
	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()
	var out []TableInfo
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		kind := "table"
		if typ == "VIEW" {
			kind = "view"
		}
		out = append(out, TableInfo{Name: name, Type: kind})
	}
	return out, rows.Err()
}

// Columns lists columns of a table, with primary key flags.
func Columns(ctx context.Context, db *sql.DB, dbType config.DBType, schema, table string) ([]ColumnInfo, error) {
	if schema == "" || table == "" {
		return nil, fmt.Errorf("schema and table are required")
	}
	query := pgColumnsQuery()
	args := []any{schema, table}
	if dbType == config.MySQL {
		query = mysqlColumnsQuery()
		args = []any{schema, table, schema, table}
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}
	defer rows.Close()
	var out []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullable string
		var isPK bool
		if err := rows.Scan(&c.Name, &c.DataType, &nullable, &isPK); err != nil {
			return nil, err
		}
		c.Nullable = strings.EqualFold(nullable, "YES")
		c.IsPrimaryKey = isPK
		c.DataType = strings.ToLower(c.DataType)
		out = append(out, c)
	}
	return out, rows.Err()
}

func pgColumnsQuery() string {
	return `SELECT c.column_name, c.data_type, c.is_nullable,
		(pk.column_name IS NOT NULL) AS is_pk
	FROM information_schema.columns c
	LEFT JOIN (
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_schema = $1
		  AND tc.table_name = $2
	) pk ON pk.column_name = c.column_name
	WHERE c.table_schema = $1 AND c.table_name = $2
	ORDER BY c.ordinal_position`
}

func mysqlColumnsQuery() string {
	return `SELECT c.column_name, c.data_type, c.is_nullable,
		(pk.column_name IS NOT NULL) AS is_pk
	FROM information_schema.columns c
	LEFT JOIN (
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_schema = ?
		  AND tc.table_name = ?
	) pk ON pk.column_name = c.column_name
	WHERE c.table_schema = ? AND c.table_name = ?
	ORDER BY c.ordinal_position`
}
