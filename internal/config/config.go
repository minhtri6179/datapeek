// Package config manages saved connections: metadata lives in
// ~/.datapeek/connections.json, passwords in the OS keychain.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// DBType identifies the database kind of a connection.
type DBType string

const (
	MySQL      DBType = "mysql"
	PostgreSQL DBType = "postgres"
)

// Connection is a saved connection profile. Password is never
// persisted to disk; it is stored in the OS keychain under keyringKey.
type Connection struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     DBType `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Database string `json:"database"`
	SSL      bool   `json:"ssl"`
	// HasPassword reports whether a password is stored in the keychain.
	HasPassword bool `json:"has_password,omitempty"`
}

// Store loads and saves connections.
type Store struct {
	dir  string
	file string
}

// NewStore returns a Store rooted at dir (usually ~/.datapeek).
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir, file: filepath.Join(dir, "connections.json")}, nil
}

// DefaultDir returns ~/.datapeek.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".datapeek"), nil
}

func (s *Store) load() ([]Connection, error) {
	data, err := os.ReadFile(s.file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var conns []Connection
	if err := json.Unmarshal(data, &conns); err != nil {
		return nil, fmt.Errorf("parse connections.json: %w", err)
	}
	return conns, nil
}

func (s *Store) save(conns []Connection) error {
	data, err := json.MarshalIndent(conns, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, data, 0o600)
}

// List returns all saved connections.
func (s *Store) List() ([]Connection, error) {
	return s.load()
}

// Save inserts or updates a connection. If password is non-empty it is
// stored in the keychain; if empty and storePassword is true, any
// existing keychain entry is deleted.
func (s *Store) Save(conn Connection, password string, clearPassword bool) error {
	conns, err := s.load()
	if err != nil {
		return err
	}
	if password != "" {
		if err := keyring.Set(service, keyringKey(conn.ID), password); err != nil {
			return fmt.Errorf("keychain write: %w", err)
		}
		conn.HasPassword = true
	} else if clearPassword {
		_ = keyring.Delete(service, keyringKey(conn.ID))
		conn.HasPassword = false
	} else {
		// preserve existing flag
		for _, c := range conns {
			if c.ID == conn.ID {
				conn.HasPassword = c.HasPassword
			}
		}
	}
	replaced := false
	for i, c := range conns {
		if c.ID == conn.ID {
			conns[i] = conn
			replaced = true
			break
		}
	}
	if !replaced {
		conns = append(conns, conn)
	}
	return s.save(conns)
}

// Delete removes a connection and its keychain secret.
func (s *Store) Delete(id string) error {
	conns, err := s.load()
	if err != nil {
		return err
	}
	out := conns[:0]
	for _, c := range conns {
		if c.ID != id {
			out = append(out, c)
		}
	}
	if len(out) != len(conns) {
		if err := s.save(out); err != nil {
			return err
		}
		_ = keyring.Delete(service, keyringKey(id))
	}
	return nil
}

// Password retrieves the stored password for a connection.
func Password(id string) (string, error) {
	return keyring.Get(service, keyringKey(id))
}

const service = "datapeek"

func keyringKey(id string) string {
	return "connection:" + id
}
