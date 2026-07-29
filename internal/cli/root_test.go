package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	buildInfo = BuildInfo{Version: "1.2.3", Commit: "abc1234", Date: "2026-01-01"}

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}

	got := out.String()
	for _, want := range []string{"1.2.3", "abc1234", "2026-01-01"} {
		if !strings.Contains(got, want) {
			t.Errorf("version output missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestRootHelp(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}

	if !strings.Contains(out.String(), "go-template") {
		t.Errorf("help output missing command name\ngot:\n%s", out.String())
	}
}
