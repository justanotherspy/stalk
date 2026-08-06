package config

import (
	"errors"
	"strings"
	"testing"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func homeDir(path string) func() (string, error) {
	return func() (string, error) { return path, nil }
}

func TestSocketPath(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "STALK_SOCKET wins",
			env: map[string]string{
				"STALK_SOCKET":    "/tmp/custom.sock",
				"XDG_RUNTIME_DIR": "/run/user/1000",
			},
			want: "/tmp/custom.sock",
		},
		{
			name: "XDG_RUNTIME_DIR",
			env:  map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"},
			want: "/run/user/1000/stalk/daemon.sock",
		},
		{
			name: "relative XDG_RUNTIME_DIR is invalid per the XDG spec",
			env:  map[string]string{"XDG_RUNTIME_DIR": "run/user/1000"},
			want: "/home/dan/.stalk/run/daemon.sock",
		},
		{
			name: "home fallback",
			env:  map[string]string{},
			want: "/home/dan/.stalk/run/daemon.sock",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := socketPath(envFrom(tt.env), homeDir("/home/dan"))
			if err != nil {
				t.Fatalf("socketPath: %v", err)
			}
			if got != tt.want {
				t.Errorf("socketPath = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSocketPathRejectsRelativeOverride(t *testing.T) {
	_, err := socketPath(envFrom(map[string]string{"STALK_SOCKET": "rel/daemon.sock"}), homeDir("/home/dan"))
	if err == nil || !strings.Contains(err.Error(), "STALK_SOCKET must be an absolute path") {
		t.Errorf("socketPath err = %v, want absolute-path error", err)
	}
}

func TestSocketPathHomeError(t *testing.T) {
	boom := errors.New("no home")
	_, err := socketPath(envFrom(nil), func() (string, error) { return "", boom })
	if !errors.Is(err, boom) {
		t.Errorf("socketPath err = %v, want wrapped %v", err, boom)
	}
}

func TestDBPath(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "XDG_STATE_HOME",
			env:  map[string]string{"XDG_STATE_HOME": "/home/dan/.local/state"},
			want: "/home/dan/.local/state/stalk/stalk.db",
		},
		{
			name: "relative XDG_STATE_HOME is invalid per the XDG spec",
			env:  map[string]string{"XDG_STATE_HOME": "state"},
			want: "/home/dan/.stalk/state/stalk.db",
		},
		{
			name: "home fallback",
			env:  map[string]string{},
			want: "/home/dan/.stalk/state/stalk.db",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dbPath(envFrom(tt.env), homeDir("/home/dan"))
			if err != nil {
				t.Fatalf("dbPath: %v", err)
			}
			if got != tt.want {
				t.Errorf("dbPath = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeDir(t *testing.T) {
	t.Setenv("STALK_SOCKET", "/tmp/stalk-test/daemon.sock")
	got, err := RuntimeDir()
	if err != nil {
		t.Fatalf("RuntimeDir: %v", err)
	}
	if got != "/tmp/stalk-test" {
		t.Errorf("RuntimeDir = %q, want %q", got, "/tmp/stalk-test")
	}
}
