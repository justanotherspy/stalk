package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Runtime and state paths are XDG-based with home-directory fallbacks for
// macOS (which sets neither XDG variable by default). Per the XDG base
// directory spec, a relative value in an XDG variable is invalid and treated
// as unset. Resolution rules live in docs/PROTOCOL.md §2.1 (socket) and
// docs/DB-SCHEMA.md §1 (database); daemon and clients must agree, so every
// process resolves through these functions.

// SocketPath resolves the daemon's unix socket path: $STALK_SOCKET if set
// (must be absolute), else $XDG_RUNTIME_DIR/stalk/daemon.sock, else
// ~/.stalk/run/daemon.sock.
func SocketPath() (string, error) {
	return socketPath(os.Getenv, os.UserHomeDir)
}

func socketPath(getenv func(string) string, home func() (string, error)) (string, error) {
	if p := getenv("STALK_SOCKET"); p != "" {
		if !filepath.IsAbs(p) {
			return "", fmt.Errorf("STALK_SOCKET must be an absolute path, got %q", p)
		}
		return filepath.Clean(p), nil
	}
	if dir := getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "stalk", "daemon.sock"), nil
	}
	h, err := home()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(h, ".stalk", "run", "daemon.sock"), nil
}

// RuntimeDir is the directory holding the socket and the daemon.lock /
// spawn.lock files (docs/PROTOCOL.md §9).
func RuntimeDir() (string, error) {
	p, err := SocketPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

// DBPath resolves the SQLite database path: $XDG_STATE_HOME/stalk/stalk.db,
// else ~/.stalk/state/stalk.db.
func DBPath() (string, error) {
	return dbPath(os.Getenv, os.UserHomeDir)
}

func dbPath(getenv func(string) string, home func() (string, error)) (string, error) {
	if dir := getenv("XDG_STATE_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "stalk", "stalk.db"), nil
	}
	h, err := home()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(h, ".stalk", "state", "stalk.db"), nil
}
