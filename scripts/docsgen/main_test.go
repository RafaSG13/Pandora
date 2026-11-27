package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/config"
)

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	// Windows checkouts may convert line endings; the page is compared as text.
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func TestCommittedFlagReferenceIsUpToDate(t *testing.T) {
	if got := repoFile(t, output); got != render() {
		t.Fatalf("%s is stale: run `go run ./scripts/docsgen` and commit the result", output)
	}
}

func TestFlagReferenceCoversEveryCommand(t *testing.T) {
	page := render()
	names := commandNames(run("help"))
	if len(names) < 10 {
		t.Fatalf("parsed only %d commands from `pandora help`: %v", len(names), names)
	}
	for _, n := range names {
		if !strings.Contains(page, "## `pandora "+n+"`") {
			t.Errorf("no section for %q", n)
		}
	}
}

func TestConfigurationPageDocumentsEveryKeyAndAction(t *testing.T) {
	page := repoFile(t, "docs/reference/configuration.md")
	for _, k := range config.Keys {
		if !strings.Contains(page, "`"+k+"`") {
			t.Errorf("docs/reference/configuration.md does not mention setting %q", k)
		}
	}
	for _, a := range config.KeyActions {
		if !strings.Contains(page, "`"+a+"`") {
			t.Errorf("docs/reference/configuration.md does not mention key action %q", a)
		}
	}
}
