package examples

import "testing"

func TestNormalizeKey(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"already_clean_KEY": "ALREADY_CLEAN_KEY",
		"Max Retry Count":   "MAX_RETRY_COUNT",
		"  spaced  out  ":   "SPACED_OUT",
		"--flag.name--":     "FLAG_NAME",
		"a..b..c":           "A_B_C",
		"123 abc":           "123_ABC",
		"!!!":               "",
	}
	for in, want := range cases {
		if got := NormalizeKey(in); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}
