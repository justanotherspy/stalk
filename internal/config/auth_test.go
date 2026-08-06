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
		name     string
		src      Source
		env      map[string]string
		gh       func() bool
		want     AuthMode
		wantNote string // substring; empty means no note at all
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
			name:     "github unset token_var falls back to gh cli with a note",
			src:      Source{Name: "gh", Type: SourceGitHub, TokenVar: "MISSING_TOKEN"},
			gh:       ghYes,
			want:     AuthGHCli,
			wantNote: "token_var MISSING_TOKEN is unset",
		},
		{
			name:     "github unset token_var falls back to unauthenticated with a note",
			src:      Source{Name: "gh", Type: SourceGitHub, TokenVar: "MISSING_TOKEN"},
			gh:       ghNo,
			want:     AuthUnauthenticated,
			wantNote: "token_var MISSING_TOKEN is unset",
		},
		{
			name:     "empty-string env value counts as unset",
			src:      Source{Name: "gh", Type: SourceGitHub, TokenVar: "EMPTY_TOKEN"},
			env:      map[string]string{"EMPTY_TOKEN": ""},
			gh:       ghNo,
			want:     AuthUnauthenticated,
			wantNote: "token_var EMPTY_TOKEN is unset",
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
		{
			name:     "http_poll unset token_var falls back to unauthenticated, gh not consulted",
			src:      Source{Name: "deploys", Type: SourceHTTPPoll, TokenVar: "MISSING_TOKEN"},
			gh:       func() bool { t.Error("ghToken consulted for http_poll"); return true },
			want:     AuthUnauthenticated,
			wantNote: "token_var MISSING_TOKEN is unset",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, note := ResolveAuthMode(&tt.src, env(tt.env), tt.gh)
			if got != tt.want {
				t.Errorf("ResolveAuthMode = %q, want %q", got, tt.want)
			}
			if tt.wantNote == "" && note != "" {
				t.Errorf("ResolveAuthMode note = %q, want none", note)
			}
			if tt.wantNote != "" && !strings.Contains(note, tt.wantNote) {
				t.Errorf("ResolveAuthMode note = %q, want it to contain %q", note, tt.wantNote)
			}
		})
	}
}
