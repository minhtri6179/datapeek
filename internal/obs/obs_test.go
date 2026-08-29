package obs

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestCorrelationID(t *testing.T) {
	id := NewCorrelationID()
	if len(id) != 8 {
		t.Fatalf("expected 8-char id, got %q", id)
	}
	if NewCorrelationID() == id {
		t.Fatal("ids should be unique")
	}
}

func TestCorrelationRoundTrip(t *testing.T) {
	ctx := WithCorrelation(context.Background(), "abcd1234")
	got, ok := CorrelationFrom(ctx)
	if !ok || got != "abcd1234" {
		t.Fatalf("expected abcd1234, got %q ok=%v", got, ok)
	}
}

func TestCorrelationAbsent(t *testing.T) {
	if _, ok := CorrelationFrom(context.Background()); ok {
		t.Fatal("expected no correlation id")
	}
}

func TestLoggerBeforeInit(t *testing.T) {
	l := Logger()
	if l == nil {
		t.Fatal("logger must never be nil")
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, slog.LevelDebug); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir, slog.LevelError); err != nil {
		t.Fatal(err)
	}
	// First level (Debug) must win since Init is a no-op after the first call.
	if !Logger().Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("expected debug level to remain enabled")
	}
}

func TestCEnrichesLogger(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, slog.LevelDebug); err != nil {
		t.Fatal(err)
	}
	// We can't easily capture output without injecting a handler; smoke-test that
	// C does not panic and returns non-nil.
	if C(WithCorrelation(context.Background(), "id1")) == nil {
		t.Fatal("C returned nil logger")
	}
}

func TestRedactionHelper(t *testing.T) {
	// The wire contract: never log SQL values. Guard with a helper used by callers.
	redacted := RedactQuery("SELECT * FROM users WHERE password = 'secret'")
	if strings.Contains(redacted, "secret") {
		t.Fatalf("value leaked: %q", redacted)
	}
}
