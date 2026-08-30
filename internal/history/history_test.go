package history

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func entry(sql string, i int) Entry {
	return Entry{
		SQL:       sql,
		Timestamp: time.Date(2026, 8, 29, 12, 0, i, 0, time.UTC),
		Success:   true,
		ElapsedMs: int64(i),
	}
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAppendAndList(t *testing.T) {
	s := newStore(t)
	if err := s.Append("c1", entry("select 1", 1)); err != nil {
		t.Fatal(err)
	}
	if err := s.Append("c1", entry("select 2", 2)); err != nil {
		t.Fatal(err)
	}
	got, err := s.List("c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].SQL != "select 2" {
		t.Fatalf("newest first: got %q", got[0].SQL)
	}
}

func TestListEmpty(t *testing.T) {
	s := newStore(t)
	got, err := s.List("missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestPerConnectionIsolation(t *testing.T) {
	s := newStore(t)
	_ = s.Append("c1", entry("select 1", 1))
	_ = s.Append("c2", entry("select 2", 2))
	got, _ := s.List("c1")
	if len(got) != 1 || got[0].SQL != "select 1" {
		t.Fatalf("c1 polluted: %+v", got)
	}
}

func TestCapAt200(t *testing.T) {
	s := newStore(t)
	for i := 0; i < MaxEntriesPerConnection+50; i++ {
		if err := s.Append("c1", entry(fmt.Sprintf("select %d", i), i)); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.List("c1")
	if len(got) != MaxEntriesPerConnection {
		t.Fatalf("expected cap %d, got %d", MaxEntriesPerConnection, len(got))
	}
	// Newest survives.
	if got[0].SQL != fmt.Sprintf("select %d", MaxEntriesPerConnection+49) {
		t.Fatalf("newest entry lost: %q", got[0].SQL)
	}
}

func TestClear(t *testing.T) {
	s := newStore(t)
	_ = s.Append("c1", entry("select 1", 1))
	if err := s.Clear("c1"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.List("c1")
	if len(got) != 0 {
		t.Fatal("expected cleared")
	}
	// Clearing an unknown connection is a no-op.
	if err := s.Clear("nope"); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptFileRecovers(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.file, []byte("not json{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Append("c1", entry("select 1", 1)); err != nil {
		t.Fatalf("corrupt history should not block appends: %v", err)
	}
	got, _ := s.List("c1")
	if len(got) != 1 {
		t.Fatalf("expected fresh history, got %d", len(got))
	}
}

func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	_ = s.Append("c1", entry("select 1", 1))
	info, err := os.Stat(s.file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600, got %v", info.Mode().Perm())
	}
}