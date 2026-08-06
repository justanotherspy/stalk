package config

import (
	"strings"
	"testing"
)

func TestResolveAuthMode(t *testing.T) {
	env := func(m map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
	}
	ghYes := func() bool { return true }
	ghNo := func() bool { return false }

	tests := []struct {
		name string
		src  Source
		env  map[string]string
		gh   func() bool
		want AuthMode
	}{
		{
			name: "github token_var set and present",
			src:  Source{Name: "gh", Type: SourceGitHub, TokenVar: "GH_TOKEN"},
			env:  map[string]string{"GH_TOKEN": "value"},
			gh:   ghNo,
			want: AuthTokenEnv,
		},
		{
			name: "github falls back to gh cli",
			src:  Source{Name: "gh", Type: SourceGitHub},
			gh:   ghYes,
			want: AuthGHCli,
		},
		{
			name: "github falls back to unauthenticated",
			src:  Source{Name: "gh", Type: SourceGitHub},
			gh:   ghNo,
			want: AuthUnauthenticated,
		},
		{
			name: "http_poll token_var set and present",
			src:  Source{Name: "deploys", Type: SourceHTTPPoll, TokenVar: "DEPLOY_TOKEN"},
			env:  map[string]string{"DEPLOY_TOKEN": "value"},
			gh:   ghNo,
			want: AuthTokenEnv,
		},
		{
			name: "http_poll without token never consults gh",
			src:  Source{Name: "deploys", Type: SourceHTTPPoll},
			gh:   func() bool { t.Error("ghToken consulted for http_poll"); return true },
			want: AuthUnauthenticated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveAuthMode(&tt.src, env(tt.env), tt.gh)
			if err != nil {
				t.Fatalf("ResolveAuthMode: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveAuthMode = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAuthModeMissingTokenVar(t *testing.T) {
	for _, src := range []Source{
		{Name: "gh", Type: SourceGitHub, TokenVar: "MISSING_TOKEN"},
		{Name: "deploys", Type: SourceHTTPPoll, TokenVar: "EMPTY_TOKEN"},
	} {
		env := func(k string) (string, bool) {
			if k == "EMPTY_TOKEN" {
				return "", true // set but empty counts as missing
			}
			return "", false
		}
		gh := func() bool { t.Error("ghToken consulted despite configured token_var"); return true }

		_, err := ResolveAuthMode(&src, env, gh)
		if err == nil {
			t.Errorf("ResolveAuthMode(%s) succeeded, want error", src.Name)
			continue
		}
		if !strings.Contains(err.Error(), src.TokenVar) || !strings.Contains(err.Error(), "is not set") {
			t.Errorf("ResolveAuthMode(%s) err = %q, want it to name %s", src.Name, err, src.TokenVar)
		}
	}
}
