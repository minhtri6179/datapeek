package query

import (
	"testing"
	"time"

	"datapeek/internal/config"
)

func TestQuoteIdentifierMySQL(t *testing.T) {
	got, err := QuoteIdentifier(config.MySQL, "users")
	if err != nil || got != "`users`" {
		t.Fatalf("got %q err %v", got, err)
	}
	got, err = QuoteIdentifier(config.MySQL, "weird`name")
	if err != nil || got != "`weird``name`" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestQuoteIdentifierPostgres(t *testing.T) {
	got, err := QuoteIdentifier(config.PostgreSQL, "users")
	if err != nil || got != `"users"` {
		t.Fatalf("got %q err %v", got, err)
	}
	got, err = QuoteIdentifier(config.PostgreSQL, `we"name`)
	if err != nil || got != `"we""name"` {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestQuoteIdentifierRejectsEmpty(t *testing.T) {
	if _, err := QuoteIdentifier(config.MySQL, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestQuoteQualifiedName(t *testing.T) {
	got, err := QuoteQualifiedName(config.MySQL, "mydb", "users")
	if err != nil || got != "`mydb`.`users`" {
		t.Fatalf("got %q err %v", got, err)
	}
	got, err = QuoteQualifiedName(config.PostgreSQL, "", "users")
	if err != nil || got != `"users"` {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestClampPagination(t *testing.T) {
	if _, _, err := clampPagination(-1, 0); err != nil {
		t.Fatal(err)
	}
	offset, size, err := clampPagination(2, 1000)
	// pageSize clamps to 500 before the offset math: 2 * 500 = 1000.
	if err != nil || offset != 1000 || size != 500 {
		t.Fatalf("offset=%d size=%d err=%v", offset, size, err)
	}
	if _, _, err := clampPagination(10_000_000, 500); err == nil {
		t.Fatal("expected offset overflow error")
	}
}

func TestSerializeValueNull(t *testing.T) {
	if got := SerializeValue(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSerializeValueTime(t *testing.T) {
	ts := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	if got := SerializeValue(ts); got != "2026-08-29T12:00:00Z" {
		t.Fatalf("got %v", got)
	}
}

func TestSerializeValueTextBytes(t *testing.T) {
	if got := SerializeValue([]byte("hello")); got != "hello" {
		t.Fatalf("got %v", got)
	}
}

func TestSerializeValueBinaryBytes(t *testing.T) {
	if got := SerializeValue([]byte{0x00, 0xff, 0x10}); got != "0x00ff10" {
		t.Fatalf("got %v", got)
	}
}

func TestSerializeValueNumbers(t *testing.T) {
	if got := SerializeValue(int64(42)); got != int64(42) {
		t.Fatalf("got %v", got)
	}
	if got := SerializeValue(3.14); got != "3.14" {
		t.Fatalf("got %v", got)
	}
	if got := SerializeValue(true); got != true {
		t.Fatalf("got %v", got)
	}
}

func TestSerializeValueString(t *testing.T) {
	if got := SerializeValue("plain"); got != "plain" {
		t.Fatalf("got %v", got)
	}
}
