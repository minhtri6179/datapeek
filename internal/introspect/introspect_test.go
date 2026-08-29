package introspect

import (
	"strings"
	"testing"

	"datapeek/internal/config"
)

// The introspect package talks to live databases; unit tests here cover
// pure logic and error paths that do not need a server.

func TestSchemasUnsupportedType(t *testing.T) {
	if _, err := Schemas(nil, nil, "redis"); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}
func TestTablesRequiresSchema(t *testing.T) {
	if _, err := Tables(nil, nil, config.MySQL, ""); err == nil {
		t.Fatal("expected error for empty schema")
	}
}

func TestColumnsRequiresSchemaAndTable(t *testing.T) {
	if _, err := Columns(nil, nil, config.MySQL, "", "users"); err == nil {
		t.Fatal("expected error for empty schema")
	}
	if _, err := Columns(nil, nil, config.MySQL, "db", ""); err == nil {
		t.Fatal("expected error for empty table")
	}
}

func TestMySQLQueryHasFourPlaceholders(t *testing.T) {
	// The MySQL variant must carry exactly 4 ? placeholders to match its
	// 4 bind args (schema, table, schema, table).
	const needle = "?"
	q := mysqlColumnsQuery()
	if got := strings.Count(q, needle); got != 4 {
		t.Fatalf("expected 4 placeholders, got %d", got)
	}
}

func TestPGQueryUsesNumberedParams(t *testing.T) {
	q := pgColumnsQuery()
	if strings.Contains(q, "?") {
		t.Fatal("pg query must not contain ? placeholders")
	}
	if strings.Count(q, "$1") != 2 || strings.Count(q, "$2") != 2 {
		t.Fatal("pg query must reuse $1 and $2")
	}
}
