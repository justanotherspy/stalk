package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// newViper builds a Viper wired the way internal/cli's initConfig wires the
// global one — STALK_ env prefix, key replacer, AutomaticEnv, defaults — and
// pointed at a temp file holding yaml. Keeping the wiring identical is the
// point: these tests must see the same precedence the CLI sees.
func newViper(t *testing.T, yaml string) *viper.Viper {
	t.Helper()
	v := viper.New()
	v.SetEnvPrefix("STALK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	SetDefaults(v)

	path := filepath.Join(t.TempDir(), "stalk.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("writing config fixture: %v", err)
	}
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("reading config fixture: %v", err)
	}
	return v
}

func loadYAML(t *testing.T, yaml string) (*Config, error) {
	t.Helper()
	return Load(newViper(t, yaml))
}

func TestLoadDefaultsWithoutFile(t *testing.T) {
	v := viper.New()
	SetDefaults(v)

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := &Config{
		Version: 1,
		Daemon: Daemon{
			IdleExitAfter:              DefaultIdleExitAfter,
			DisconnectGrace:            DefaultDisconnectGrace,
			MaxSubscriptionsPerSession: DefaultMaxSubscriptionsPerSession,
			MaxConnections:             DefaultMaxConnections,
			Retention: Retention{
				EventsMaxAge:  DefaultEventsMaxAge,
				EventsMaxRows: DefaultEventsMaxRows,
			},
		},
		Sources: map[string]Source{},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoadFullConfig(t *testing.T) {
	cfg, err := loadYAML(t, `
version: 1

daemon:
  idle_exit_after: 45m
  disconnect_grace: 5m
  max_subscriptions_per_session: 16
  max_connections: 32
  retention:
    events_max_age: 7d
    events_max_rows: 1000

sources:
  github-work:
    type: github
    token_var: GITHUB_WORK_TOKEN
    api_url: https://github.example.com/api/v3
    allowed_owners: [acme, contoso]

  github-personal:
    type: github

  deploys:
    type: http_poll
    url: https://deploys.example/api/recent.json
    interval: 90s
    items_path: deploys
    id_field: id
    token_var: DEPLOY_TOKEN

  minimal-poll:
    type: http_poll
    url: http://localhost:8080/items.json
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := &Config{
		Version: 1,
		Daemon: Daemon{
			IdleExitAfter:              45 * time.Minute,
			DisconnectGrace:            5 * time.Minute,
			MaxSubscriptionsPerSession: 16,
			MaxConnections:             32,
			Retention: Retention{
				EventsMaxAge:  7 * 24 * time.Hour,
				EventsMaxRows: 1000,
			},
		},
		Sources: map[string]Source{
			"github-work": {
				Name:          "github-work",
				Type:          SourceGitHub,
				TokenVar:      "GITHUB_WORK_TOKEN",
				APIURL:        "https://github.example.com/api/v3",
				AllowedOwners: []string{"acme", "contoso"},
			},
			"github-personal": {
				Name: "github-personal",
				Type: SourceGitHub,
			},
			"deploys": {
				Name:      "deploys",
				Type:      SourceHTTPPoll,
				URL:       "https://deploys.example/api/recent.json",
				Interval:  90 * time.Second,
				ItemsPath: "deploys",
				IDField:   "id",
				TokenVar:  "DEPLOY_TOKEN",
			},
			"minimal-poll": {
				Name:     "minimal-poll",
				Type:     SourceHTTPPoll,
				URL:      "http://localhost:8080/items.json",
				Interval: DefaultHTTPPollInterval,
			},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	t.Setenv("STALK_DAEMON_IDLE_EXIT_AFTER", "2h")

	cfg, err := loadYAML(t, "daemon:\n  idle_exit_after: 45m\n")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.Daemon.IdleExitAfter, 2*time.Hour; got != want {
		t.Errorf("IdleExitAfter = %s, want %s (env must beat file)", got, want)
	}
}

func TestLoadBadEnvValue(t *testing.T) {
	t.Setenv("STALK_DAEMON_DISCONNECT_GRACE", "soon")

	_, err := loadYAML(t, "")
	if err == nil || !strings.Contains(err.Error(), "daemon.disconnect_grace") {
		t.Errorf("Load() err = %v, want daemon.disconnect_grace duration error", err)
	}
}

// STALK_VERSION is install.sh's release pin; a user exporting it must not
// break config parsing. The version key is a file-schema fact and is read
// from the file layer only.
func TestLoadIgnoresStalkVersionEnv(t *testing.T) {
	t.Setenv("STALK_VERSION", "v9.9.9")

	for _, yaml := range []string{"", "version: 1\n"} {
		cfg, err := loadYAML(t, yaml)
		if err != nil {
			t.Errorf("Load(%q) with STALK_VERSION set: %v", yaml, err)
			continue
		}
		if cfg.Version != 1 {
			t.Errorf("Load(%q).Version = %d, want 1", yaml, cfg.Version)
		}
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "unsupported version",
			yaml:    "version: 2\n",
			wantErr: "unsupported config version 2",
		},
		{
			name:    "non-numeric version",
			yaml:    "version: two\n",
			wantErr: "unsupported config version two",
		},
		{
			name:    "daemon not a mapping",
			yaml:    "daemon: 30m\n",
			wantErr: "daemon: expected a mapping",
		},
		{
			name:    "unknown daemon key",
			yaml:    "daemon:\n  disconect_grace: 10m\n",
			wantErr: `daemon: unknown key "disconect_grace"`,
		},
		{
			name:    "retention not a mapping",
			yaml:    "daemon:\n  retention: 14d\n",
			wantErr: "daemon.retention: expected a mapping",
		},
		{
			name:    "unknown retention key",
			yaml:    "daemon:\n  retention:\n    events_max_days: 14\n",
			wantErr: `daemon.retention: unknown key "events_max_days"`,
		},
		{
			name:    "bad daemon duration",
			yaml:    "daemon:\n  idle_exit_after: 45x\n",
			wantErr: `daemon.idle_exit_after: bad duration "45x"`,
		},
		{
			name:    "unitless daemon duration",
			yaml:    "daemon:\n  idle_exit_after: 1800\n",
			wantErr: "daemon.idle_exit_after: bad duration \"1800\": missing unit",
		},
		{
			name:    "negative daemon duration",
			yaml:    "daemon:\n  disconnect_grace: -10m\n",
			wantErr: "daemon.disconnect_grace: must be positive",
		},
		{
			name:    "bad retention duration",
			yaml:    "daemon:\n  retention:\n    events_max_age: 14x\n",
			wantErr: `daemon.retention.events_max_age: bad duration "14x"`,
		},
		{
			name:    "non-integer rows cap",
			yaml:    "daemon:\n  retention:\n    events_max_rows: many\n",
			wantErr: "daemon.retention.events_max_rows: not a whole number",
		},
		{
			name:    "zero rows cap",
			yaml:    "daemon:\n  retention:\n    events_max_rows: 0\n",
			wantErr: "daemon.retention.events_max_rows: must be positive",
		},
		{
			name:    "negative connection cap",
			yaml:    "daemon:\n  max_connections: -1\n",
			wantErr: "daemon.max_connections: must be positive",
		},
		{
			name:    "sources not a mapping",
			yaml:    "sources: [github]\n",
			wantErr: "sources: expected a mapping",
		},
		{
			name:    "source not a mapping",
			yaml:    "sources:\n  gh: github\n",
			wantErr: `source "gh": expected a mapping of settings`,
		},
		{
			name:    "source missing type",
			yaml:    "sources:\n  gh:\n    token_var: TOKEN\n",
			wantErr: `source "gh": missing type`,
		},
		{
			name:    "unknown source type",
			yaml:    "sources:\n  queue:\n    type: sqs\n",
			wantErr: `source "queue": unknown type "sqs" (valid types: github, http_poll)`,
		},
		{
			name:    "invalid source name",
			yaml:    "sources:\n  \"-bad\":\n    type: github\n",
			wantErr: `source name "-bad"`,
		},
		{
			name:    "unknown github key",
			yaml:    "sources:\n  gh:\n    type: github\n    interval: 30s\n",
			wantErr: `source "gh": unknown key "interval"`,
		},
		{
			name:    "bad token_var name",
			yaml:    "sources:\n  gh:\n    type: github\n    token_var: \"my token\"\n",
			wantErr: `source "gh": token_var "my token" is not a valid environment variable name`,
		},
		{
			name:    "bad api_url",
			yaml:    "sources:\n  gh:\n    type: github\n    api_url: not-a-url\n",
			wantErr: `source "gh": api_url must be an http(s) URL`,
		},
		{
			name:    "allowed_owners not a list",
			yaml:    "sources:\n  gh:\n    type: github\n    allowed_owners: acme/repo\n",
			wantErr: `source "gh": allowed_owners: "acme/repo" is not a valid owner name`,
		},
		{
			name:    "allowed_owners bad element",
			yaml:    "sources:\n  gh:\n    type: github\n    allowed_owners: [\"a/b\"]\n",
			wantErr: `source "gh": allowed_owners: "a/b" is not a valid owner name`,
		},
		{
			name:    "allowed_owners empty list",
			yaml:    "sources:\n  gh:\n    type: github\n    allowed_owners: []\n",
			wantErr: `source "gh": allowed_owners: expected a non-empty list`,
		},
		{
			name:    "http_poll missing url",
			yaml:    "sources:\n  deploys:\n    type: http_poll\n",
			wantErr: `source "deploys": missing url`,
		},
		{
			name:    "http_poll non-http url",
			yaml:    "sources:\n  deploys:\n    type: http_poll\n    url: ftp://example.com/x\n",
			wantErr: `source "deploys": url must be an http(s) URL`,
		},
		{
			name:    "http_poll interval below floor",
			yaml:    "sources:\n  deploys:\n    type: http_poll\n    url: https://example.com/x\n    interval: 5s\n",
			wantErr: `source "deploys": interval 5s is below the 10s minimum`,
		},
		{
			name:    "http_poll bad interval",
			yaml:    "sources:\n  deploys:\n    type: http_poll\n    url: https://example.com/x\n    interval: fast\n",
			wantErr: `source "deploys": interval: bad duration "fast"`,
		},
		{
			name:    "http_poll unknown key",
			yaml:    "sources:\n  deploys:\n    type: http_poll\n    url: https://example.com/x\n    allowed_owners: [acme]\n",
			wantErr: `source "deploys": unknown key "allowed_owners"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadYAML(t, tt.yaml)
			if err == nil {
				t.Fatalf("Load() succeeded, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() err = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
