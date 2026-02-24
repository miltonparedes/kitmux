package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

const (
	cacheDir  = ".config/kitmux"
	cacheFile = "sessions-cache.json"
	version   = 1
)

// Snapshot holds the cached session data persisted to disk.
type Snapshot struct {
	Version   int                 `json:"version"`
	UpdatedAt time.Time           `json:"updated_at"`
	Sessions  []tmux.Session      `json:"sessions"`
	RepoRoots map[string]string   `json:"repo_roots"`
	Stats     map[string]DiffStat `json:"stats,omitempty"`
	StatsTTL  time.Time           `json:"stats_ttl,omitempty"`
}

// DiffStat holds working tree diff stats for a session.
type DiffStat struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
}

// Load reads the cached snapshot from disk. Returns nil if not found or invalid.
func Load() *Snapshot {
	data, err := os.ReadFile(filePath())
	if err != nil {
		return nil
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil
	}
	if snap.Version != version {
		return nil
	}
	return &snap
}

// Save persists the snapshot to disk.
func Save(snap *Snapshot) error {
	snap.Version = version
	snap.UpdatedAt = time.Now()

	p := filePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// StatsValid reports whether the cached stats are still within their TTL.
func (s *Snapshot) StatsValid() bool {
	if s == nil || len(s.Stats) == 0 {
		return false
	}
	return time.Now().Before(s.StatsTTL)
}

func filePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, cacheDir, cacheFile)
}
