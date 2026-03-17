package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/miltonparedes/kitmux/internal/tmux"
)

const (
	legacySessionCacheFile = "sessions-cache.json"
	metaUpdatedAt          = "session_cache.updated_at"
	metaRepoRootsAt        = "session_cache.repo_roots_refreshed_at"
	metaStatsTTL           = "session_cache.stats_ttl"
	tableWorkspaces        = "workspaces"
	tableSessionSnapshots  = "session_snapshots"
	tableRepoRoots         = "repo_roots"
	tableWorktreeStats     = "worktree_stats"
)

// DiffStat is the persisted worktree stat record.
type DiffStat struct {
	RepoRoot string
	Added    int
	Deleted  int
}

// SessionCache is the persisted session snapshot.
type SessionCache struct {
	UpdatedAt            time.Time
	Sessions             []tmux.Session
	RepoRoots            map[string]string
	RepoRootsRefreshedAt time.Time
	Stats                map[string]DiffStat
	StatsTTL             time.Time
}

type legacySessionCachePayload struct {
	Version              int                 `json:"version"`
	UpdatedAt            time.Time           `json:"updated_at"`
	Sessions             []tmux.Session      `json:"sessions"`
	RepoRoots            map[string]string   `json:"repo_roots"`
	RepoRootsRefreshedAt time.Time           `json:"repo_roots_refreshed_at,omitempty"`
	Stats                map[string]DiffStat `json:"stats,omitempty"`
	StatsTTL             time.Time           `json:"stats_ttl,omitempty"`
}

func LoadSessionCache() (*SessionCache, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	if err := importLegacySessionCache(db); err != nil {
		return nil, err
	}

	return loadSessionCache(db)
}

func SaveSessionCache(snap *SessionCache) error {
	if snap == nil {
		return nil
	}

	db, err := open()
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin save session cache: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM session_snapshots`); err != nil {
		return fmt.Errorf("clear session snapshots: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM repo_roots`); err != nil {
		return fmt.Errorf("clear repo roots: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM worktree_stats`); err != nil {
		return fmt.Errorf("clear worktree stats: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM metadata WHERE key IN (?, ?, ?)`, metaUpdatedAt, metaRepoRootsAt, metaStatsTTL); err != nil {
		return fmt.Errorf("clear session cache metadata: %w", err)
	}

	now := time.Now()
	updatedAt := snap.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}

	if err := insertSessions(tx, snap.Sessions, updatedAt); err != nil {
		return err
	}
	if err := insertRepoRoots(tx, snap.RepoRoots, snap.RepoRootsRefreshedAt); err != nil {
		return err
	}
	if err := insertWorktreeStats(tx, snap.RepoRoots, snap.Stats, snap.StatsTTL, now); err != nil {
		return err
	}
	if err := setMetaTime(tx, metaUpdatedAt, updatedAt); err != nil {
		return err
	}
	if err := setMetaTime(tx, metaRepoRootsAt, snap.RepoRootsRefreshedAt); err != nil {
		return err
	}
	if err := setMetaTime(tx, metaStatsTTL, snap.StatsTTL); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save session cache: %w", err)
	}
	return nil
}

func loadSessionCache(db *sql.DB) (*SessionCache, error) {
	snap := &SessionCache{}

	updatedAt, err := getMetaTime(db, metaUpdatedAt)
	if err != nil {
		return nil, err
	}
	repoRootsRefreshedAt, err := getMetaTime(db, metaRepoRootsAt)
	if err != nil {
		return nil, err
	}
	statsTTL, err := getMetaTime(db, metaStatsTTL)
	if err != nil {
		return nil, err
	}

	sessions, err := loadSessions(db)
	if err != nil {
		return nil, err
	}
	repoRoots, err := loadRepoRoots(db)
	if err != nil {
		return nil, err
	}
	stats, err := loadWorktreeStats(db)
	if err != nil {
		return nil, err
	}

	found := len(sessions) > 0 || len(repoRoots) > 0 || len(stats) > 0 || !updatedAt.IsZero() || !repoRootsRefreshedAt.IsZero() || !statsTTL.IsZero()
	if !found {
		return nil, nil
	}

	snap.UpdatedAt = updatedAt
	snap.Sessions = sessions
	snap.RepoRoots = repoRoots
	snap.RepoRootsRefreshedAt = repoRootsRefreshedAt
	snap.Stats = stats
	snap.StatsTTL = statsTTL
	return snap, nil
}

func importLegacySessionCache(db *sql.DB) error {
	empty, err := sessionCacheEmpty(db)
	if err != nil || !empty {
		return err
	}

	data, err := os.ReadFile(legacySessionCachePath()) //nolint:gosec // path derives from the user's home dir
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read legacy session cache: %w", err)
	}

	payload, ok := decodeLegacySessionCache(data)
	if !ok {
		return nil
	}
	if payload.Version != 1 {
		return nil
	}

	return SaveSessionCache(&SessionCache{
		UpdatedAt:            payload.UpdatedAt,
		Sessions:             payload.Sessions,
		RepoRoots:            payload.RepoRoots,
		RepoRootsRefreshedAt: payload.RepoRootsRefreshedAt,
		Stats:                payload.Stats,
		StatsTTL:             payload.StatsTTL,
	})
}

func decodeLegacySessionCache(data []byte) (legacySessionCachePayload, bool) {
	var payload legacySessionCachePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return legacySessionCachePayload{}, false
	}
	return payload, true
}

func loadSessions(db *sql.DB) ([]tmux.Session, error) {
	rows, err := db.Query(`SELECT session_name, windows, attached, path, activity FROM session_snapshots ORDER BY session_name`)
	if err != nil {
		return nil, fmt.Errorf("query session snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []tmux.Session
	for rows.Next() {
		var s tmux.Session
		var attached int
		if err := rows.Scan(&s.Name, &s.Windows, &attached, &s.Path, &s.Activity); err != nil {
			return nil, fmt.Errorf("scan session snapshot: %w", err)
		}
		s.Attached = attached == 1
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session snapshots: %w", err)
	}
	return sessions, nil
}

func loadRepoRoots(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT session_name, repo_root FROM repo_roots`)
	if err != nil {
		return nil, fmt.Errorf("query repo roots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	repoRoots := make(map[string]string)
	for rows.Next() {
		var sessionName, repoRoot string
		if err := rows.Scan(&sessionName, &repoRoot); err != nil {
			return nil, fmt.Errorf("scan repo root: %w", err)
		}
		repoRoots[sessionName] = repoRoot
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repo roots: %w", err)
	}
	return repoRoots, nil
}

func loadWorktreeStats(db *sql.DB) (map[string]DiffStat, error) {
	rows, err := db.Query(`SELECT session_name, repo_root, added, deleted FROM worktree_stats`)
	if err != nil {
		return nil, fmt.Errorf("query worktree stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	stats := make(map[string]DiffStat)
	for rows.Next() {
		var sessionName string
		var stat DiffStat
		if err := rows.Scan(&sessionName, &stat.RepoRoot, &stat.Added, &stat.Deleted); err != nil {
			return nil, fmt.Errorf("scan worktree stat: %w", err)
		}
		stats[sessionName] = stat
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worktree stats: %w", err)
	}
	return stats, nil
}

func insertSessions(tx *sql.Tx, sessions []tmux.Session, updatedAt time.Time) error {
	stmt, err := tx.Prepare(`INSERT INTO session_snapshots(session_name, path, windows, attached, activity, updated_at) VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert session snapshot: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	updatedUnix := updatedAt.UnixNano()
	for _, s := range sessions {
		attached := 0
		if s.Attached {
			attached = 1
		}
		if _, err := stmt.Exec(s.Name, s.Path, s.Windows, attached, s.Activity, updatedUnix); err != nil {
			return fmt.Errorf("insert session snapshot %q: %w", s.Name, err)
		}
	}
	return nil
}

func insertRepoRoots(tx *sql.Tx, repoRoots map[string]string, refreshedAt time.Time) error {
	stmt, err := tx.Prepare(`INSERT INTO repo_roots(session_name, repo_root, refreshed_at) VALUES(?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert repo root: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	refreshedUnix := refreshedAt.UnixNano()
	for sessionName, repoRoot := range repoRoots {
		if _, err := stmt.Exec(sessionName, repoRoot, refreshedUnix); err != nil {
			return fmt.Errorf("insert repo root %q: %w", sessionName, err)
		}
	}
	return nil
}

func insertWorktreeStats(tx *sql.Tx, repoRoots map[string]string, stats map[string]DiffStat, expiresAt, updatedAt time.Time) error {
	stmt, err := tx.Prepare(`INSERT INTO worktree_stats(session_name, repo_root, added, deleted, expires_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert worktree stat: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	expiresUnix := expiresAt.UnixNano()
	updatedUnix := updatedAt.UnixNano()
	for sessionName, stat := range stats {
		repoRoot := stat.RepoRoot
		if repoRoot == "" {
			repoRoot = repoRoots[sessionName]
		}
		if _, err := stmt.Exec(sessionName, repoRoot, stat.Added, stat.Deleted, expiresUnix, updatedUnix); err != nil {
			return fmt.Errorf("insert worktree stat %q: %w", sessionName, err)
		}
	}
	return nil
}

func sessionCacheEmpty(db *sql.DB) (bool, error) {
	tables := []string{tableSessionSnapshots, tableRepoRoots, tableWorktreeStats}
	for _, table := range tables {
		empty, err := tableEmpty(db, table)
		if err != nil {
			return false, err
		}
		if !empty {
			return false, nil
		}
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM metadata WHERE key IN (?, ?, ?)`, metaUpdatedAt, metaRepoRootsAt, metaStatsTTL).Scan(&count); err != nil {
		return false, fmt.Errorf("count session cache metadata: %w", err)
	}
	return count == 0, nil
}

func tableEmpty(db *sql.DB, table string) (bool, error) {
	var count int
	query := tableCountQuery(table)
	if query == "" {
		return false, fmt.Errorf("unsupported table %s", table)
	}
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return false, fmt.Errorf("count table %s: %w", table, err)
	}
	return count == 0, nil
}

func tableCountQuery(table string) string {
	switch table {
	case tableWorkspaces:
		return `SELECT COUNT(*) FROM workspaces`
	case tableSessionSnapshots:
		return `SELECT COUNT(*) FROM session_snapshots`
	case tableRepoRoots:
		return `SELECT COUNT(*) FROM repo_roots`
	case tableWorktreeStats:
		return `SELECT COUNT(*) FROM worktree_stats`
	default:
		return ""
	}
}

func setMetaTime(tx *sql.Tx, key string, value time.Time) error {
	encoded := "0"
	if !value.IsZero() {
		encoded = strconv.FormatInt(value.UnixNano(), 10)
	}
	if _, err := tx.Exec(`INSERT INTO metadata(key, value) VALUES(?, ?)`, key, encoded); err != nil {
		return fmt.Errorf("set metadata %q: %w", key, err)
	}
	return nil
}

func getMetaTime(db *sql.DB, key string) (time.Time, error) {
	var raw string
	err := db.QueryRow(`SELECT value FROM metadata WHERE key = ?`, key).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("get metadata %q: %w", key, err)
	}
	ns, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse metadata %q: %w", key, err)
	}
	if ns == 0 {
		return time.Time{}, nil
	}
	return time.Unix(0, ns), nil
}

func legacySessionCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, legacySessionCacheFile)
}
