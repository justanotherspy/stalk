package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const day = 24 * time.Hour

// ParseDuration parses a config duration: anything time.ParseDuration accepts,
// plus a leading whole-day component with the unit "d" (docs/MVP-SPEC.md §4
// uses "14d" for retention). "14d" is 14 days; "1d12h" is a day and a half.
// A bare number is rejected — a unit is always required, so "30" can never be
// misread as nanoseconds.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("bad duration %q (examples: 30s, 5m, 2h, 14d)", s)
	}

	if i := strings.IndexByte(s, 'd'); i > 0 && allDigits(s[:i]) {
		days, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, fmt.Errorf("bad duration %q (examples: 30s, 5m, 2h, 14d)", s)
		}
		var tail time.Duration
		if rest := s[i+1:]; rest != "" {
			tail, err = time.ParseDuration(rest)
			if err != nil {
				return 0, fmt.Errorf("bad duration %q (examples: 30s, 5m, 2h, 14d)", s)
			}
		}
		return time.Duration(days)*day + tail, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		if _, numErr := strconv.Atoi(s); numErr == nil {
			return 0, fmt.Errorf("bad duration %q: missing unit (use s, m, h, or d — e.g. \"30s\", \"14d\")", s)
		}
		return 0, fmt.Errorf("bad duration %q (examples: 30s, 5m, 2h, 14d)", s)
	}
	return d, nil
}

// FormatDuration renders d in the syntax ParseDuration accepts, preferring the
// day unit when it divides evenly ("336h0m0s" becomes "14d") and trimming the
// zero components time.Duration.String is fond of ("30m0s" becomes "30m").
func FormatDuration(d time.Duration) string {
	if d >= day && d%day == 0 {
		return fmt.Sprintf("%dd", d/day)
	}
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
