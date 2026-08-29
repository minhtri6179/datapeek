package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListEmpty(t *testing.T) {
	s := newTestStore(t)
	conns, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 0 {
		t.Fatalf("expected empty list, got %d", len(conns))
	}
}

func TestSaveAndList(t *testing.T) {
	s := newTestStore(t)
	c := Connection{ID: "c1", Name: "local", Type: MySQL, Host: "127.0.0.1", Port: 3306, User: "root", Database: "test"}
	if err := s.Save(c, "", false); err != nil {
		t.Fatal(err)
	}
	conns, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].ID != "c1" || conns[0].Name != "local" {
		t.Fatalf("unexpected: %+v", conns)
	}
}

func TestSaveUpsert(t *testing.T) {
	s := newTestStore(t)
	c := Connection{ID: "c1", Name: "local", Type: MySQL, Host: "127.0.0.1", Port: 3306}
	if err := s.Save(c, "", false); err != nil {
		t.Fatal(err)
	}
	c.Name = "renamed"
	c.Port = 3307
	if err := s.Save(c, "", false); err != nil {
		t.Fatal(err)
	}
	conns, _ := s.List()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].Name != "renamed" || conns[0].Port != 3307 {
		t.Fatalf("upsert failed: %+v", conns[0])
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	c := Connection{ID: "c1", Name: "local", Type: PostgreSQL, Host: "localhost", Port: 5432}
	if err := s.Save(c, "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("c1"); err != nil {
		t.Fatal(err)
	}
	conns, _ := s.List()
	if len(conns) != 0 {
		t.Fatalf("expected deletion, got %d", len(conns))
	}
}

func TestDeleteMissingIsNoop(t *testing.T) {
	s := newTestStore(t)
	if err := s.Delete("nope"); err != nil {
		t.Fatal(err)
	}
}

func TestFilePermissions(t *testing.T) {
	s := newTestStore(t)
	c := Connection{ID: "c1", Name: "local", Type: MySQL}
	if err := s.Save(c, "", false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(s.file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600, got %v", info.Mode().Perm())
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
