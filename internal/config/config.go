// Package config parses and validates stalk's configuration.
//
// It builds on the template's Viper stack rather than replacing it: the CLI
// resolves the file (--config, default ~/.stalk.yaml), the STALK_ env prefix,
// and flag bindings, and this package turns the resolved values into typed,
// validated structs. Two sections are stalk's own — `daemon` and `sources` —
// documented in docs/MVP-SPEC.md §4.
//
// token_var values are the NAMES of environment variables, resolved by the
// daemon at poll time. They are never read into Viper, so tokens never enter
// config precedence and can never be printed by accident.
package config

import (
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

// Defaults for the daemon section (docs/MVP-SPEC.md §4, docs/PROTOCOL.md §2.3).
const (
	DefaultIdleExitAfter              = 30 * time.Minute
	DefaultDisconnectGrace            = 10 * time.Minute
	DefaultMaxSubscriptionsPerSession = 32
	DefaultMaxConnections             = 64
	DefaultEventsMaxAge               = 14 * 24 * time.Hour
	DefaultEventsMaxRows              = 50000

	// DefaultHTTPPollInterval applies when an http_poll source omits interval;
	// MinHTTPPollInterval is the floor enforced at validation time.
	DefaultHTTPPollInterval = 30 * time.Second
	MinHTTPPollInterval     = 10 * time.Second
)

// Config is the validated stalk configuration.
type Config struct {
	// Version is the config file schema version (1 when the file omits it).
	Version int
	Daemon  Daemon
	Sources map[string]Source
}

// Daemon holds the daemon section (docs/MVP-SPEC.md §4).
type Daemon struct {
	IdleExitAfter              time.Duration
	DisconnectGrace            time.Duration
	MaxSubscriptionsPerSession int
	MaxConnections             int
	Retention                  Retention
}

// Retention holds the daemon.retention section (docs/DB-SCHEMA.md §6).
type Retention struct {
	EventsMaxAge  time.Duration
	EventsMaxRows int
}

// SourceType names a source implementation.
type SourceType string

// The source types implemented in v1 (docs/MVP-SPEC.md §5).
const (
	SourceGitHub   SourceType = "github"
	SourceHTTPPoll SourceType = "http_poll"
)

// Source is one named entry of the sources map. Fields beyond Name, Type, and
// TokenVar apply to a single type only; parsing rejects them elsewhere.
type Source struct {
	Name string
	Type SourceType

	// TokenVar is the name of an environment variable holding a credential.
	// Only the name is stored; the daemon reads the value at poll time.
	TokenVar string

	// github only.
	APIURL        string
	AllowedOwners []string

	// http_poll only.
	URL       string
	Interval  time.Duration
	ItemsPath string
	IDField   string
}

// SetDefaults registers the daemon defaults on v so that STALK_ environment
// overrides and `stalk config check` output work even without a config file.
// Durations are registered as strings in the same syntax the file uses.
func SetDefaults(v *viper.Viper) {
	v.SetDefault("daemon.idle_exit_after", "30m")
	v.SetDefault("daemon.disconnect_grace", "10m")
	v.SetDefault("daemon.max_subscriptions_per_session", DefaultMaxSubscriptionsPerSession)
	v.SetDefault("daemon.max_connections", DefaultMaxConnections)
	v.SetDefault("daemon.retention.events_max_age", "14d")
	v.SetDefault("daemon.retention.events_max_rows", DefaultEventsMaxRows)
}

var (
	envVarNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	sourceNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// Load builds a validated Config from v, which must already have read its
// config file (if any) and had SetDefaults applied.
//
// Scalar daemon settings come from v, so the template's precedence holds:
// flags > STALK_ env > config file > defaults. Structural concerns — the
// version key, unknown-key detection, and the sources map — are read from the
// config file layer alone: env vars cannot define nested sources anyway, and
// STALK_VERSION is install.sh's version pin, which must not shadow the file's
// schema version. Unknown top-level keys are not rejected, because the top
// level belongs to Viper (the template's verbose/log keys live there, and
// later PRs add more).
func Load(v *viper.Viper) (*Config, error) {
	fv, err := fileLayer(v.ConfigFileUsed())
	if err != nil {
		return nil, err
	}

	cfg := &Config{Version: 1}

	if raw := fv.Get("version"); raw != nil {
		n, castErr := cast.ToIntE(raw)
		if castErr != nil || n != 1 {
			return nil, fmt.Errorf("version: unsupported config version %v (this stalk understands version 1)", raw)
		}
		cfg.Version = n
	}

	if err := validateDaemonKeys(fv); err != nil {
		return nil, err
	}
	if err := loadDaemon(v, &cfg.Daemon); err != nil {
		return nil, err
	}

	sources, err := loadSources(fv)
	if err != nil {
		return nil, err
	}
	cfg.Sources = sources

	return cfg, nil
}

// fileLayer returns a bare Viper holding only the config file's contents, or
// an empty one when no file was used. It carries no env bindings, defaults, or
// flags, so its values are exactly what the file says.
func fileLayer(path string) (*viper.Viper, error) {
	fv := viper.New()
	if path == "" {
		return fv, nil
	}
	fv.SetConfigFile(path)
	if err := fv.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}
	return fv, nil
}

// sectionMap extracts a mapping-valued key from raw file data. A missing or
// null key returns an empty map; any other non-mapping value is an error.
func sectionMap(fv *viper.Viper, key string) (map[string]any, error) {
	raw := fv.Get(key)
	if raw == nil {
		return map[string]any{}, nil
	}
	m, err := cast.ToStringMapE(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: expected a mapping, got %v", key, raw)
	}
	return m, nil
}

// rejectUnknown fails on the first key of m (in sorted order, for stable
// errors) that is not in allowed.
func rejectUnknown(section string, m map[string]any, allowed ...string) error {
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if !slices.Contains(allowed, k) {
			return fmt.Errorf("%s: unknown key %q (known keys: %s)", section, k, strings.Join(allowed, ", "))
		}
	}
	return nil
}

// validateDaemonKeys checks the daemon section's structure as written in the
// file: every key must be one stalk understands, so a typo like
// `disconect_grace` fails loudly instead of silently using the default.
func validateDaemonKeys(fv *viper.Viper) error {
	daemon, err := sectionMap(fv, "daemon")
	if err != nil {
		return err
	}
	if err := rejectUnknown("daemon", daemon,
		"idle_exit_after", "disconnect_grace", "max_subscriptions_per_session",
		"max_connections", "retention"); err != nil {
		return err
	}
	if _, ok := daemon["retention"]; !ok {
		return nil
	}
	retention, err := cast.ToStringMapE(daemon["retention"])
	if err != nil {
		return fmt.Errorf("daemon.retention: expected a mapping, got %v", daemon["retention"])
	}
	return rejectUnknown("daemon.retention", retention, "events_max_age", "events_max_rows")
}

// loadDaemon reads the daemon scalars through v, honoring the full precedence
// chain, and validates each one.
func loadDaemon(v *viper.Viper, d *Daemon) error {
	var err error
	if d.IdleExitAfter, err = durationSetting(v, "daemon.idle_exit_after"); err != nil {
		return err
	}
	if d.DisconnectGrace, err = durationSetting(v, "daemon.disconnect_grace"); err != nil {
		return err
	}
	if d.MaxSubscriptionsPerSession, err = intSetting(v, "daemon.max_subscriptions_per_session"); err != nil {
		return err
	}
	if d.MaxConnections, err = intSetting(v, "daemon.max_connections"); err != nil {
		return err
	}
	if d.Retention.EventsMaxAge, err = durationSetting(v, "daemon.retention.events_max_age"); err != nil {
		return err
	}
	if d.Retention.EventsMaxRows, err = intSetting(v, "daemon.retention.events_max_rows"); err != nil {
		return err
	}
	return nil
}

// durationSetting resolves key through v and parses it as a positive duration.
func durationSetting(v *viper.Viper, key string) (time.Duration, error) {
	raw := v.Get(key)
	s, err := cast.ToStringE(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: bad duration %v: value must be a string like \"30m\" or \"14d\"", key, raw)
	}
	d, err := ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: must be positive, got %q", key, s)
	}
	return d, nil
}

// intSetting resolves key through v and parses it as a positive integer.
func intSetting(v *viper.Viper, key string) (int, error) {
	raw := v.Get(key)
	n, err := cast.ToIntE(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: not a whole number: %v", key, raw)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s: must be positive, got %d", key, n)
	}
	return n, nil
}

// loadSources parses and validates the sources map from the file layer.
func loadSources(fv *viper.Viper) (map[string]Source, error) {
	m, err := sectionMap(fv, "sources")
	if err != nil {
		return nil, err
	}
	sources := make(map[string]Source, len(m))
	for _, name := range slices.Sorted(maps.Keys(m)) {
		src, err := parseSource(name, m[name])
		if err != nil {
			return nil, err
		}
		sources[name] = src
	}
	return sources, nil
}

func parseSource(name string, raw any) (Source, error) {
	if !sourceNameRE.MatchString(name) {
		return Source{}, fmt.Errorf("source name %q: must start with a letter or digit and contain only letters, digits, - and _", name)
	}
	sec := fmt.Sprintf("source %q", name)

	m, err := cast.ToStringMapE(raw)
	if err != nil || raw == nil {
		return Source{}, fmt.Errorf("%s: expected a mapping of settings", sec)
	}

	rawType, ok := m["type"]
	if !ok {
		return Source{}, fmt.Errorf("%s: missing type (valid types: %s, %s)", sec, SourceGitHub, SourceHTTPPoll)
	}
	typ, err := cast.ToStringE(rawType)
	if err != nil {
		return Source{}, fmt.Errorf("%s: type: expected a string, got %v", sec, rawType)
	}

	src := Source{Name: name, Type: SourceType(typ)}
	switch src.Type {
	case SourceGitHub:
		err = parseGitHubSource(sec, m, &src)
	case SourceHTTPPoll:
		err = parseHTTPPollSource(sec, m, &src)
	default:
		err = fmt.Errorf("%s: unknown type %q (valid types: %s, %s)", sec, typ, SourceGitHub, SourceHTTPPoll)
	}
	if err != nil {
		return Source{}, err
	}
	return src, nil
}

func parseGitHubSource(sec string, m map[string]any, src *Source) error {
	if err := rejectUnknown(sec, m, "type", "token_var", "api_url", "allowed_owners"); err != nil {
		return err
	}
	var err error
	if src.TokenVar, err = tokenVarField(sec, m); err != nil {
		return err
	}
	if src.APIURL, err = stringField(sec, m, "api_url"); err != nil {
		return err
	}
	if src.APIURL != "" {
		if err := validateHTTPURL(sec, "api_url", src.APIURL); err != nil {
			return err
		}
	}
	return parseAllowedOwners(sec, m, src)
}

func parseHTTPPollSource(sec string, m map[string]any, src *Source) error {
	if err := rejectUnknown(sec, m, "type", "url", "interval", "items_path", "id_field", "token_var"); err != nil {
		return err
	}
	var err error
	if src.URL, err = stringField(sec, m, "url"); err != nil {
		return err
	}
	if src.URL == "" {
		return fmt.Errorf("%s: missing url", sec)
	}
	if err := validateHTTPURL(sec, "url", src.URL); err != nil {
		return err
	}
	if src.Interval, err = intervalField(sec, m); err != nil {
		return err
	}
	if src.ItemsPath, err = stringField(sec, m, "items_path"); err != nil {
		return err
	}
	if src.IDField, err = stringField(sec, m, "id_field"); err != nil {
		return err
	}
	if src.TokenVar, err = tokenVarField(sec, m); err != nil {
		return err
	}
	return nil
}

// intervalField parses http_poll's poll interval, defaulting when absent and
// enforcing the 10s floor (docs/MVP-SPEC.md §4).
func intervalField(sec string, m map[string]any) (time.Duration, error) {
	raw, ok := m["interval"]
	if !ok || raw == nil {
		return DefaultHTTPPollInterval, nil
	}
	s, err := cast.ToStringE(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: interval: bad duration %v: value must be a string like \"30s\"", sec, raw)
	}
	d, err := ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%s: interval: %w", sec, err)
	}
	if d < MinHTTPPollInterval {
		return 0, fmt.Errorf("%s: interval %s is below the %s minimum", sec, FormatDuration(d), FormatDuration(MinHTTPPollInterval))
	}
	return d, nil
}

// tokenVarField validates that token_var, when present, is a plausible
// environment variable name. Only the name is kept — never a value.
func tokenVarField(sec string, m map[string]any) (string, error) {
	s, err := stringField(sec, m, "token_var")
	if err != nil {
		return "", err
	}
	if s != "" && !envVarNameRE.MatchString(s) {
		return "", fmt.Errorf("%s: token_var %q is not a valid environment variable name", sec, s)
	}
	return s, nil
}

func parseAllowedOwners(sec string, m map[string]any, src *Source) error {
	raw, ok := m["allowed_owners"]
	if !ok || raw == nil {
		return nil
	}
	owners, err := cast.ToStringSliceE(raw)
	if err != nil || len(owners) == 0 {
		return fmt.Errorf("%s: allowed_owners: expected a non-empty list of owner names, got %v", sec, raw)
	}
	for _, o := range owners {
		if o == "" || strings.ContainsAny(o, "/ \t") {
			return fmt.Errorf("%s: allowed_owners: %q is not a valid owner name", sec, o)
		}
	}
	src.AllowedOwners = owners
	return nil
}

// stringField extracts an optional string key from a source mapping.
func stringField(sec string, m map[string]any, key string) (string, error) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return "", nil
	}
	s, err := cast.ToStringE(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %s: expected a string, got %v", sec, key, raw)
	}
	return s, nil
}

// validateHTTPURL requires an absolute http or https URL with a host.
func validateHTTPURL(sec, key, s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("%s: %s: not a valid URL: %w", sec, key, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s: %s must be an http(s) URL, got %q", sec, key, s)
	}
	if u.Host == "" {
		return fmt.Errorf("%s: %s has no host: %q", sec, key, s)
	}
	return nil
}
