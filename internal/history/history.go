package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MaxEntriesPerConnection caps stored queries per connection.
const MaxEntriesPerConnection = 200

// Entry is one executed console query.
type Entry struct {
	SQL       string    `json:"sql"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	ElapsedMs int64     `json:"elapsedMs"`
	Error     string    `json:"error,omitempty"`
}

// Store persists history entries keyed by connection id.
type Store struct {
	mu   sync.Mutex
	file string
}

// NewStore creates a Store writing to dir/history.json.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{file: filepath.Join(dir, "history.json")}, nil
}

func (s *Store) load() (map[string][]Entry, error) {
	data, err := os.ReadFile(s.file)
	if os.IsNotExist(err) {
		return map[string][]Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string][]Entry{}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string][]Entry{}, nil
	}
	return out, nil
}

func (s *Store) save(m map[string][]Entry) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, data, 0o600)
}

// Append records an entry for connID at the front of its list.
func (s *Store) Append(connID string, e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return err
	}
	list := append([]Entry{e}, m[connID]...)
	if len(list) > MaxEntriesPerConnection {
		list = list[:MaxEntriesPerConnection]
	}
	m[connID] = list
	return s.save(m)
}

// List returns entries for connID, newest first.
func (s *Store) List(connID string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return nil, err
	}
	return m[connID], nil
}

// Clear removes all entries for connID.
func (s *Store) Clear(connID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return err
	}
	if _, exists := m[connID]; !exists {
		return nil
	}
	delete(m, connID)
	return s.save(m)
}