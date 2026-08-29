// Package obs provides local-first structured logging for datapeek.
// Logs are written as JSON to rotating files under ~/.datapeek/logs/.
// SQL values, parameters, and credentials must never be logged.
package obs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type ctxKey string

const correlationKey ctxKey = "correlation_id"

var (
	mu     sync.Mutex
	logger *slog.Logger
)

// Init sets up the JSON logger writing to the given directory.
// It is safe to call once at startup; subsequent calls are no-ops.
func Init(dir string, level slog.Level) error {
	mu.Lock()
	defer mu.Unlock()
	if logger != nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	w := &lumberjack.Logger{
		Filename:   filepath.Join(dir, "datapeek.log"),
		MaxSize:    10, // MB
		MaxBackups: 5,
		MaxAge:     7, // days
		Compress:   true,
	}
	logger = slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.UTC().Format(time.RFC3339Nano))
				}
			}
			return a
		},
	}))
	return nil
}

// Logger returns the active logger, or a discarding logger before Init.
func Logger() *slog.Logger {
	mu.Lock()
	defer mu.Unlock()
	if logger == nil {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	return logger
}

// NewCorrelationID returns a random 8-hex-char correlation id.
func NewCorrelationID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// WithCorrelation attaches a correlation id to the context so that
// every log line emitted with it can be traced back to a single UI action.
func WithCorrelation(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey, id)
}

// CorrelationFrom extracts the correlation id from ctx, if present.
func CorrelationFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(correlationKey).(string)
	return id, ok
}

// C returns a logger enriched with the correlation id from ctx.
func C(ctx context.Context) *slog.Logger {
	l := Logger()
	if id, ok := CorrelationFrom(ctx); ok {
		return l.With(slog.String("correlation_id", id))
	}
	return l
}

// RedactQuery replaces string literals in SQL text with ? so that
// values never reach the log file. Table and column names are kept.
func RedactQuery(sql string) string {
	var b strings.Builder
	inString := false
	for _, r := range sql {
		switch {
		case r == '\'':
			if inString {
				b.WriteByte('?')
				inString = false
			} else {
				inString = true
			}
		case inString:
			// swallow characters inside the literal
		default:
			b.WriteRune(r)
		}
	}
	if inString { // unterminated literal
		b.WriteByte('?')
	}
	return b.String()
}
