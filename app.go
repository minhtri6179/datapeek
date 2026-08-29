package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"datapeek/internal/config"
	"datapeek/internal/connection"
	"datapeek/internal/introspect"
	"datapeek/internal/obs"
	"datapeek/internal/query"
)

// App is the Wails binding surface. Every method validates input, sets
// up a correlation id, and logs the call without ever logging secrets,
// SQL values, or query results.
type App struct {
	ctx     context.Context
	store   *config.Store
	manager *connection.Manager
}

// NewApp creates the app. Configure data dir overrides for tests.
func NewApp() *App {
	dir, err := config.DefaultDir()
	if err != nil {
		dir = "."
	}
	return newAppAt(dir)
}

func newAppAt(dir string) *App {
	store, err := config.NewStore(dir)
	if err != nil {
		panic(fmt.Sprintf("config store: %v", err))
	}
	return &App{
		store:   store,
		manager: connection.NewManager(),
	}
}

// startup initializes logging and saves the context.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dir, err := config.DefaultDir()
	if err != nil {
		dir = "."
	}
	if err := obs.Init(filepath.Join(dir, "logs"), slog.LevelInfo); err != nil {
		// Logging is best-effort; the app still runs.
		fmt.Printf("obs init: %v\n", err)
	}
	obs.C(ctx).Info("app started")
}

// shutdown closes all database pools.
func (a *App) shutdown(ctx context.Context) {
	a.manager.CloseAll()
	obs.C(ctx).Info("app stopped")
}

// ---- Connections ----

// TestConnection attempts to open and ping a connection without saving it.
func (a *App) TestConnection(cfg config.Connection, password string) error {
	if err := validateConn(cfg); err != nil {
		return err
	}
	m := connection.NewManager()
	defer m.CloseAll()
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	_, err := m.Open(ctx, cfg, password)
	return err
}

// SaveConnection saves (or updates) a connection profile.
func (a *App) SaveConnection(cfg config.Connection, password string, clearPassword bool) error {
	if err := validateConn(cfg); err != nil {
		return err
	}
	if cfg.ID == "" {
		cfg.ID = obs.NewCorrelationID()
	}
	err := a.store.Save(cfg, password, clearPassword)
	a.logDB("save_connection", cfg.ID, err)
	return err
}

// DeleteConnection removes a saved connection and its secret.
func (a *App) DeleteConnection(id string) error {
	err := a.store.Delete(id)
	a.manager.Close(id)
	a.logDB("delete_connection", id, err)
	return err
}

// ListConnections returns all saved profiles (without passwords).
func (a *App) ListConnections() ([]config.Connection, error) {
	return a.store.List()
}

// Connect opens a live pool for a saved connection.
func (a *App) Connect(id string) error {
	conns, err := a.store.List()
	if err != nil {
		return err
	}
	var cfg *config.Connection
	for i := range conns {
		if conns[i].ID == id {
			cfg = &conns[i]
			break
		}
	}
	if cfg == nil {
		return fmt.Errorf("connection %s not found", id)
	}
	password := ""
	if cfg.HasPassword {
		password, err = config.Password(id)
		if err != nil {
			// Saved with password flag but the keychain entry is gone.
			cfg.HasPassword = false
		}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	_, err = a.manager.Open(ctx, *cfg, password)
	a.logDB("connect", id, err)
	return err
}

// Disconnect closes the pool for a connection.
func (a *App) Disconnect(id string) {
	a.manager.Close(id)
	a.logDB("disconnect", id, nil)
}

// ---- Schema browsing ----

// GetSchemas lists user schemas/databases for a connection.
func (a *App) GetSchemas(connID string) ([]string, error) {
	db, cfg, err := a.liveConn(connID)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	out, err := introspect.Schemas(a.ctx, db, cfg.Type)
	a.logSlow("get_schemas", connID, start, err)
	return out, err
}

// GetTables lists tables and views in a schema.
func (a *App) GetTables(connID, schema string) ([]introspect.TableInfo, error) {
	db, cfg, err := a.liveConn(connID)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	out, err := introspect.Tables(a.ctx, db, cfg.Type, schema)
	a.logSlow("get_tables", connID, start, err, "schema", schema)
	return out, err
}

// GetColumns lists columns of a table.
func (a *App) GetColumns(connID, schema, table string) ([]introspect.ColumnInfo, error) {
	db, cfg, err := a.liveConn(connID)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	out, err := introspect.Columns(a.ctx, db, cfg.Type, schema, table)
	a.logSlow("get_columns", connID, start, err, "schema", schema, "table", table)
	return out, err
}

// ---- Data ----

// ReadTable returns one page of rows for the datagrid.
func (a *App) ReadTable(connID, schema, table string, page, pageSize int, sort *query.SortSpec) (*query.QueryResult, error) {
	db, cfg, err := a.liveConn(connID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	start := time.Now()
	res, err := query.ReadTable(ctx, db, cfg.Type, schema, table, page, pageSize, sort)
	a.logSlow("read_table", connID, start, err, "schema", schema, "table", table, "page", page)
	return res, err
}

// ---- Frontend telemetry ----

// LogFrontendEvent lets the React layer funnel UI errors into the same
// structured log pipeline as the backend.
func (a *App) LogFrontendEvent(level, msg, correlationID string) {
	l := obs.C(obs.WithCorrelation(a.ctx, correlationID))
	switch level {
	case "error":
		l.Error("frontend: " + msg)
	case "warn":
		l.Warn("frontend: " + msg)
	default:
		l.Info("frontend: " + msg)
	}
}

// ---- helpers ----

// liveConn resolves an open pool plus its saved profile.
func (a *App) liveConn(connID string) (*sql.DB, *config.Connection, error) {
	conns, err := a.store.List()
	if err != nil {
		return nil, nil, err
	}
	for _, c := range conns {
		if c.ID == connID {
			db, err := a.manager.Get(connID)
			return db, &c, err
		}
	}
	return nil, nil, fmt.Errorf("connection %s not found", connID)
}

func validateConn(c config.Connection) error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch c.Type {
	case config.MySQL, config.PostgreSQL:
	default:
		return fmt.Errorf("unsupported database type: %s", c.Type)
	}
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	if c.User == "" {
		return fmt.Errorf("user is required")
	}
	if c.Database == "" {
		return fmt.Errorf("database is required")
	}
	return nil
}

func (a *App) logDB(op, connID string, err error) {
	l := obs.C(a.ctx).With(slog.String("op", op), slog.String("conn", connID))
	if err != nil {
		l.Warn("db operation failed", slog.String("error", err.Error()))
		return
	}
	l.Info("db operation ok")
}

func (a *App) logSlow(op, connID string, start time.Time, err error, kv ...any) {
	elapsed := time.Since(start)
	attrs := append([]any{
		slog.String("op", op),
		slog.String("conn", connID),
		slog.Duration("elapsed", elapsed),
	}, kv...)
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		obs.C(a.ctx).Warn("operation failed", attrs...)
		return
	}
	if elapsed > 500*time.Millisecond {
		obs.C(a.ctx).Warn("slow operation", attrs...)
		return
	}
	obs.C(a.ctx).Info("operation ok", attrs...)
}
