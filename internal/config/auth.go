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
// the gh CLI can supply a token; it is consulted only for github sources with
// no token_var configured.
//
// A configured token_var whose variable is unset or empty is an error, not a
// silent fallback: the user asked for that credential, and the likely cause is
// the daemon-environment caveat (the variable must be exported in the login
// shell, not per-session).
func ResolveAuthMode(src *Source, lookupEnv func(string) (string, bool), ghToken func() bool) (AuthMode, error) {
	if src.TokenVar != "" {
		if val, ok := lookupEnv(src.TokenVar); !ok || val == "" {
			return "", fmt.Errorf("source %q: token_var %s is not set in the environment (export it in your login shell so the daemon inherits it)", src.Name, src.TokenVar)
		}
		return AuthTokenEnv, nil
	}
	if src.Type == SourceGitHub && ghToken() {
		return AuthGHCli, nil
	}
	return AuthUnauthenticated, nil
}
