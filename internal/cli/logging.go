package cli

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

// loggerKey is the (unexported, collision-free) context key under which the
// configured *slog.Logger is stored. Commands retrieve it with loggerFrom so
// that logging flows through the same logger the root command set up.
type loggerKey struct{}

// withLogger returns a copy of ctx carrying l. A nil ctx is treated as
// context.Background so callers never have to guard against it.
func withLogger(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

// loggerFrom returns the logger stored on ctx by the root command, falling back
// to slog.Default() when none is present (e.g. in tests that build a command
// directly). It never returns nil.
func loggerFrom(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
			return l
		}
	}
	return slog.Default()
}

// setupLogging builds a *slog.Logger from the resolved configuration. Values
// come from viper, so they honor the precedence wired up in initConfig:
// flags > GO_TEMPLATE_ env vars > config file > defaults. The --verbose shortcut
// forces debug level regardless of --log-level. Logs are written to w (stderr),
// keeping stdout clean for the command's actual output.
func setupLogging(w io.Writer) *slog.Logger {
	level := parseLevel(viper.GetString("log.level"))
	if viper.GetBool("verbose") {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if strings.EqualFold(viper.GetString("log.format"), "json") {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}

	return slog.New(h)
}

// parseLevel maps a human-friendly level name to a slog.Level, defaulting to
// info for empty or unrecognized values.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
