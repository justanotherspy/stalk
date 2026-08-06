package config

import (
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"30s", 30 * time.Second},
		{"45m", 45 * time.Minute},
		{"1h30m", 90 * time.Minute},
		{"1d", 24 * time.Hour},
		{"14d", 14 * 24 * time.Hour},
		{"1d12h", 36 * time.Hour},
		{" 30m ", 30 * time.Minute},
		{"1500ms", 1500 * time.Millisecond},
		{"106751d", 106751 * 24 * time.Hour}, // largest representable whole-day count
	}
	for _, tt := range tests {
		got, err := ParseDuration(tt.in)
		if err != nil {
			t.Errorf("ParseDuration(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDuration(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestParseDurationErrors(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "bad duration"},
		{"fast", "bad duration"},
		{"45x", "bad duration"},
		{"d", "bad duration"},
		{"1800", "missing unit"},
		{"14dd", "bad duration"},
		{"1.5d", "bad duration"},
		{"-14d", "bad duration"},
		{"1d-5h", "negative component"},
		// int64 overflow must be rejected, never wrapped: some wrapped values
		// come out small and POSITIVE and would pass every downstream check.
		{"106752d", "too large"},
		{"213504d", "too large"},
		{"300000d", "too large"},
		{"106751991167301d", "too large"},
		{"99999999999999999999d", "too large"},
		{"106751d24h", "too large"},
	}
	for _, tt := range tests {
		_, err := ParseDuration(tt.in)
		if err == nil {
			t.Errorf("ParseDuration(%q) succeeded, want error", tt.in)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ParseDuration(%q) err = %q, want it to contain %q", tt.in, err, tt.want)
		}
	}
}

func TestParseFormatRoundTrip(t *testing.T) {
	// FormatDuration must render every value in syntax ParseDuration accepts.
	for _, d := range []time.Duration{
		30 * time.Second,
		30 * time.Minute,
		2 * time.Hour,
		36 * time.Hour,
		14 * 24 * time.Hour,
		90 * time.Second,
	} {
		back, err := ParseDuration(FormatDuration(d))
		if err != nil {
			t.Errorf("ParseDuration(FormatDuration(%s) = %q): %v", d, FormatDuration(d), err)
			continue
		}
		if back != d {
			t.Errorf("round trip %s -> %q -> %s", d, FormatDuration(d), back)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{30 * time.Minute, "30m"},
		{2 * time.Hour, "2h"},
		{90 * time.Minute, "1h30m"},
		{14 * 24 * time.Hour, "14d"},
		{36 * time.Hour, "36h"},
		{90 * time.Second, "1m30s"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.in); got != tt.want {
			t.Errorf("FormatDuration(%s) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
