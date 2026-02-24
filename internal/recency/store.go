package recency

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	configDir = ".config/kitmux"
	storeFile = "recency.json"
)

var mu sync.Mutex

// Store holds timestamps of last usage for sessions and palette commands.
type Store struct {
	Commands map[string]time.Time `json:"commands"`
}

// Load reads the recency store from disk. Returns an empty store if not found.
func Load() *Store {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked()
}

// RecordCommand records a palette command execution timestamp.
func RecordCommand(id string) {
	mu.Lock()
	defer mu.Unlock()
	s := loadLocked()
	if s.Commands == nil {
		s.Commands = make(map[string]time.Time)
	}
	s.Commands[id] = time.Now()
	_ = saveLocked(s)
}

// SortByRecency reorders items so that recently-used ones appear first,
// preserving the original order for items without recency data.
// getKey extracts the recency key from an item at position i.
func SortByRecency[T any](items []T, timestamps map[string]time.Time, getKey func(T) string) []T {
	if len(timestamps) == 0 || len(items) == 0 {
		return items
	}

	type entry struct {
		item  T
		ts    time.Time
		index int // original position for stable sort
	}

	var recent, rest []entry
	for i, item := range items {
		key := getKey(item)
		if ts, ok := timestamps[key]; ok {
			recent = append(recent, entry{item: item, ts: ts, index: i})
		} else {
			rest = append(rest, entry{item: item, index: i})
		}
	}

	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].ts.After(recent[j].ts)
	})

	result := make([]T, 0, len(items))
	for _, e := range recent {
		result = append(result, e.item)
	}
	for _, e := range rest {
		result = append(result, e.item)
	}
	return result
}

func loadLocked() *Store {
	data, err := os.ReadFile(filePath())
	if err != nil {
		return &Store{}
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return &Store{}
	}
	return &s
}

func saveLocked(s *Store) error {
	p := filePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), storeFile+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, p) //nolint:gosec // tmpPath is created by os.CreateTemp in the destination directory
}

func filePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, storeFile)
}
