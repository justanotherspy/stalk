package config

import "fmt"

// AuthMode is how a source would authenticate, as reported by
// `stalk config check` and the protocol's sources.list — always the mode,
// never a credential (docs/PROTOCOL.md §6.6, §10.1).
type AuthMode string

// The auth modes v1 resolves (docs/MVP-SPEC.md §2: token env var, `gh auth`
// chain fallback, unauthenticated fallback).
const (
	AuthTokenEnv        AuthMode = "token_env"
	AuthGHCli           AuthMode = "gh_cli"
	AuthUnauthenticated AuthMode = "unauthenticated"
)

// ResolveAuthMode reports how the daemon would authenticate src if it polled
// right now. lookupEnv is os.LookupEnv (injected for tests) and is used only
// to test presence — values never leave this function. ghToken reports whether
// the gh CLI can supply a token; it is consulted only for github sources that
// did not resolve a token from the environment.
//
// A configured token_var whose variable is unset or empty falls back down the
// auth chain, exactly as MVP-SPEC §4 specifies (unset/empty → gh auth chain →
// unauthenticated). The returned note flags the fallback so `stalk config
// check` can surface it — the likely cause is the daemon-environment caveat
// (the variable must be exported in the login shell, not per-session).
func ResolveAuthMode(src *Source, lookupEnv func(string) (string, bool), ghToken func() bool) (mode AuthMode, note string) {
	if src.TokenVar != "" {
		if val, ok := lookupEnv(src.TokenVar); ok && val != "" {
			return AuthTokenEnv, ""
		}
		note = fmt.Sprintf("token_var %s is unset; export it in your login shell so the daemon inherits it", src.TokenVar)
	}
	if src.Type == SourceGitHub && ghToken() {
		return AuthGHCli, note
	}
	return AuthUnauthenticated, note
}
