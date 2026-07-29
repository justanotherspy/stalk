// Command gen-docs renders shell completions and man pages for the CLI.
//
// It is a development-only helper and is deliberately NOT shipped in releases
// (GoReleaser builds only ./cmd/go-template). The release pipeline and the
// `make completions` / `make man` targets run it to populate the ./completions
// and ./man directories, which are then bundled into the release archives.
//
// Usage:
//
//	go run ./cmd/gen-docs            # writes ./completions and ./man
//	go run ./cmd/gen-docs -out dist  # writes dist/completions and dist/man
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/justanotherspy/go-template/internal/cli"
)

func main() {
	out := flag.String("out", ".", "directory to write completions/ and man/ into")
	flag.Parse()

	root := cli.NewRootCmd()
	// Deterministic output: no "auto generated ... on <date>" footer that would
	// otherwise churn the man pages on every run.
	root.DisableAutoGenTag = true

	if err := genCompletions(root, filepath.Join(*out, "completions")); err != nil {
		log.Fatalf("gen-docs: completions: %v", err)
	}
	if err := genManPages(root, filepath.Join(*out, "man")); err != nil {
		log.Fatalf("gen-docs: man pages: %v", err)
	}
}

// genCompletions writes one completion script per supported shell.
func genCompletions(root *cobra.Command, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	bin := root.Name()
	if err := root.GenBashCompletionFileV2(filepath.Join(dir, bin+".bash"), true); err != nil {
		return err
	}
	if err := root.GenZshCompletionFile(filepath.Join(dir, "_"+bin)); err != nil {
		return err
	}
	if err := root.GenFishCompletionFile(filepath.Join(dir, bin+".fish"), true); err != nil {
		return err
	}
	return root.GenPowerShellCompletionFile(filepath.Join(dir, bin+".ps1"))
}

// genManPages writes a section-1 man page per command.
func genManPages(root *cobra.Command, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	hdr := &doc.GenManHeader{
		Title:   strings.ToUpper(root.Name()),
		Section: "1",
		Source:  root.Name(),
	}
	return doc.GenManTree(root, hdr, dir)
}
