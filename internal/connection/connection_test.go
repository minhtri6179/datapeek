package connection

import (
	"strings"
	"testing"

	"datapeek/internal/config"
)

func TestDSNMySQL(t *testing.T) {
	c := config.Connection{Type: config.MySQL, Host: "127.0.0.1", Port: 3306, User: "root", Database: "appdb"}
	driver, dsn, err := DSN(c, "pw")
	if err != nil {
		t.Fatal(err)
	}
	if driver != "mysql" {
		t.Fatalf("driver: %s", driver)
	}
	want := "root:pw@tcp(127.0.0.1:3306)/appdb?parseTime=true&loc=UTC&timeout=5s&readTimeout=30s&writeTimeout=30s&tls=false"
	if dsn != want {
		t.Fatalf("dsn:\n got %s\nwant %s", dsn, want)
	}
}

func TestDSNMySQLSSL(t *testing.T) {
	c := config.Connection{Type: config.MySQL, Host: "db.example.com", Port: 3306, User: "u", Database: "d", SSL: true}
	_, dsn, err := DSN(c, "x")
	if err != nil {
		t.Fatal(err)
	}
	if dsn[len(dsn)-len("&tls=true"):] != "&tls=true" {
		t.Fatalf("expected tls=true, got %s", dsn)
	}
}

func TestDSNPostgres(t *testing.T) {
	c := config.Connection{Type: config.PostgreSQL, Host: "localhost", Port: 5432, User: "postgres", Database: "appdb"}
	driver, dsn, err := DSN(c, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if driver != "pgx" {
		t.Fatalf("driver: %s", driver)
	}
	want := "host=localhost port=5432 user=postgres password=secret dbname=appdb sslmode=disable connect_timeout=5"
	if dsn != want {
		t.Fatalf("dsn:\n got %s\nwant %s", dsn, want)
	}
}

func TestDSNPostgresSSL(t *testing.T) {
	c := config.Connection{Type: config.PostgreSQL, Host: "db.example.com", Port: 5432, User: "u", Database: "d", SSL: true}
	_, dsn, err := DSN(c, "x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "sslmode=require") || strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("expected sslmode=require, got %s", dsn)
	}
}

func TestDSNUnsupported(t *testing.T) {
	c := config.Connection{Type: "redis"}
	if _, _, err := DSN(c, ""); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestManagerGetNotOpen(t *testing.T) {
	m := NewManager()
	if _, err := m.Get("nope"); err == nil {
		t.Fatal("expected error for unopened connection")
	}
	if m.IsOpen("nope") {
		t.Fatal("IsOpen should be false")
	}
}

func TestManagerCloseAllNoop(t *testing.T) {
	m := NewManager()
	m.CloseAll() // must not panic
	m.Close("missing")
}
