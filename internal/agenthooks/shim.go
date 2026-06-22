package agenthooks

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

func installAgentEventShim(home string) (string, bool, error) {
	path := filepath.Join(home, ".config", "kitmux", "hooks", "agent-event")
	changed, err := writeFileIfChanged(path, []byte(agentEventShimScript(kitmuxCommand())), 0o700)
	return path, changed, err
}

func writeFileIfChanged(path string, content []byte, perm os.FileMode) (bool, error) {
	return withFileLock(path, func() (bool, error) {
		// #nosec G304 -- path is derived from the user's home directory and a fixed kitmux config path.
		existing, err := os.ReadFile(path)
		if err == nil && bytes.Equal(existing, content) {
			info, statErr := os.Stat(path)
			if statErr == nil && info.Mode().Perm() == perm {
				return false, nil
			}
			if statErr != nil {
				return false, statErr
			}
			// #nosec G302 -- hook shims are executable only by the current user.
			return true, os.Chmod(path, perm)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		if err := writeFileAtomic(path, content, perm); err != nil {
			return false, err
		}
		return true, nil
	})
}

func agentEventShimScript(kitmux string) string {
	return "#!/bin/sh\n" +
		"set -eu\n" +
		"kitmux=" + hookShellQuote(kitmux) + "\n" +
		"if [ \"$kitmux\" = kitmux ]; then\n" +
		"  kitmux=$(command -v kitmux 2>/dev/null || printf '%s' kitmux)\n" +
		"fi\n" +
		"exec \"$kitmux\" hook agent-event \"$@\"\n"
}
