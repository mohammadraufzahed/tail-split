package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config is the persisted user configuration.
type Config struct {
	// Enabled globally enables/disables exclusions without clearing entries.
	Enabled bool `json:"enabled"`
	// Entries are the things to keep off the exit node.
	// Each is an IPv4 address ("1.2.3.4"), CIDR ("10.0.0.0/8") or hostname ("example.com").
	Entries []Entry `json:"entries"`
}

// Entry is one exclusion rule.
type Entry struct {
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "tail-split", "config.json")
}

// Store manages config state and persistence.
type Store struct {
	mu   sync.Mutex
	cfg  Config
	path string
}

func NewStore() *Store {
	return &Store{path: configPath(), cfg: Config{Enabled: true}}
}

func (s *Store) Load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &s.cfg)
}

func (s *Store) saveLocked() {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0o644)
}

func (s *Store) Path() string { return s.path }

func (s *Store) Config() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *Store) SetEnabled(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Enabled = on
	s.saveLocked()
}

func (s *Store) AddEntry(value string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.cfg.Entries {
		if e.Value == value {
			return false
		}
	}
	s.cfg.Entries = append(s.cfg.Entries, Entry{Value: value, Enabled: true})
	s.saveLocked()
	return true
}

func (s *Store) RemoveEntry(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.cfg.Entries[:0]
	for _, e := range s.cfg.Entries {
		if e.Value != value {
			out = append(out, e)
		}
	}
	s.cfg.Entries = out
	s.saveLocked()
}

func (s *Store) ToggleEntry(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.cfg.Entries {
		if e.Value == value {
			s.cfg.Entries[i].Enabled = !e.Enabled
			break
		}
	}
	s.saveLocked()
}
