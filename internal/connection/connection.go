// Package connection manages live database connections.
// One *sql.DB pool per open connection id.
package connection

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"datapeek/internal/config"
)

// Manager tracks open database pools keyed by connection id.
type Manager struct {
	mu    sync.RWMutex
	pools map[string]*sql.DB
}

// NewManager creates an empty manager.
func NewManager() *Manager {
	return &Manager{pools: make(map[string]*sql.DB)}
}

// DSN builds the driver data-source name for a connection profile.
func DSN(c config.Connection, password string) (driver string, dsn string, err error) {
	switch c.Type {
	case config.MySQL:
		driver = "mysql"
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=UTC&timeout=5s&readTimeout=30s&writeTimeout=30s",
			c.User, password, c.Host, c.Port, c.Database)
		if c.SSL {
			dsn += "&tls=true"
		} else {
			dsn += "&tls=false"
		}
		return driver, dsn, nil
	case config.PostgreSQL:
		driver = "pgx"
		sslmode := "disable"
		if c.SSL {
			sslmode = "require"
		}
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
			c.Host, c.Port, c.User, password, c.Database, sslmode)
		return driver, dsn, nil
	default:
		return "", "", fmt.Errorf("unsupported database type: %s", c.Type)
	}
}

// Open creates (or returns the existing) pool for the connection.
func (m *Manager) Open(ctx context.Context, c config.Connection, password string) (*sql.DB, error) {
	m.mu.RLock()
	if db, ok := m.pools[c.ID]; ok {
		m.mu.RUnlock()
		return db, nil
	}
	m.mu.RUnlock()

	driver, dsn, err := DSN(c, password)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect %s@%s:%d: %w", c.User, c.Host, c.Port, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.pools[c.ID]; ok { // concurrent open race
		db.Close()
		return existing, nil
	}
	m.pools[c.ID] = db
	return db, nil
}

// Get returns the open pool for id.
func (m *Manager) Get(id string) (*sql.DB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	db, ok := m.pools[id]
	if !ok {
		return nil, fmt.Errorf("connection %s is not open", id)
	}
	return db, nil
}

// Close closes the pool for id, if open.
func (m *Manager) Close(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if db, ok := m.pools[id]; ok {
		db.Close()
		delete(m.pools, id)
	}
}

// CloseAll closes every open pool (app shutdown).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, db := range m.pools {
		db.Close()
		delete(m.pools, id)
	}
}

// IsOpen reports whether id has an open pool.
func (m *Manager) IsOpen(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.pools[id]
	return ok
}
