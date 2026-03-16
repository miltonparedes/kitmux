package cache

import (
	"sync"
	"time"

	"github.com/miltonparedes/kitmux/internal/store"
	"github.com/miltonparedes/kitmux/internal/tmux"
)

const (
	cacheDir  = ".config/kitmux"
	cacheFile = "sessions-cache.json"
	version   = 1
)

var mu sync.Mutex

// Snapshot holds the cached session data persisted to disk.
type Snapshot struct {
	Version              int                 `json:"version"`
	UpdatedAt            time.Time           `json:"updated_at"`
	Sessions             []tmux.Session      `json:"sessions"`
	RepoRoots            map[string]string   `json:"repo_roots"`
	RepoRootsRefreshedAt time.Time           `json:"repo_roots_refreshed_at,omitempty"`
	Stats                map[string]DiffStat `json:"stats,omitempty"`
	StatsTTL             time.Time           `json:"stats_ttl,omitempty"`
}

// DiffStat holds working tree diff stats for a session.
type DiffStat struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
}

// Load reads the cached snapshot from disk. Returns nil if not found or invalid.
func Load() *Snapshot {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked()
}

func Save(snap *Snapshot) error {
	mu.Lock()
	defer mu.Unlock()
	return saveLocked(snap)
}

func Update(updateFn func(*Snapshot)) error {
	mu.Lock()
	defer mu.Unlock()

	snap := loadLocked()
	if snap == nil {
		snap = &Snapshot{}
	}
	updateFn(snap)
	return saveLocked(snap)
}

func loadLocked() *Snapshot {
	persisted, err := store.LoadSessionCache()
	if err != nil {
		return nil
	}
	if persisted == nil {
		return nil
	}

	stats := make(map[string]DiffStat, len(persisted.Stats))
	for sessionName, stat := range persisted.Stats {
		stats[sessionName] = DiffStat{Added: stat.Added, Deleted: stat.Deleted}
	}

	return &Snapshot{
		Version:              version,
		UpdatedAt:            persisted.UpdatedAt,
		Sessions:             persisted.Sessions,
		RepoRoots:            persisted.RepoRoots,
		RepoRootsRefreshedAt: persisted.RepoRootsRefreshedAt,
		Stats:                stats,
		StatsTTL:             persisted.StatsTTL,
	}
}

func saveLocked(snap *Snapshot) error {
	snap.Version = version
	snap.UpdatedAt = time.Now()

	persisted := &store.SessionCache{
		UpdatedAt:            snap.UpdatedAt,
		Sessions:             snap.Sessions,
		RepoRoots:            snap.RepoRoots,
		RepoRootsRefreshedAt: snap.RepoRootsRefreshedAt,
		StatsTTL:             snap.StatsTTL,
	}
	if len(snap.Stats) > 0 {
		persisted.Stats = make(map[string]store.DiffStat, len(snap.Stats))
		for sessionName, stat := range snap.Stats {
			persisted.Stats[sessionName] = store.DiffStat{Added: stat.Added, Deleted: stat.Deleted}
		}
	}

	return store.SaveSessionCache(persisted)
}

// StatsValid reports whether the cached stats are still within their TTL.
func (s *Snapshot) StatsValid() bool {
	if s == nil || len(s.Stats) == 0 {
		return false
	}
	return time.Now().Before(s.StatsTTL)
}
