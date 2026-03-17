package store

import (
	"os"
	"path/filepath"
	"testing"
)

func useTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func stateDBPath(home string) string {
	return filepath.Join(home, configDir, databaseFile)
}

func legacyWorkspacesJSONPath(home string) string {
	return filepath.Join(home, configDir, legacyWorkspacesFile)
}

func legacySessionCacheJSONPath(home string) string {
	return filepath.Join(home, configDir, legacySessionCacheFile)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
