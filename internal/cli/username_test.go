package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/store"
)

func TestAddUsernameShowsInListJSON(t *testing.T) {
	isolate(t)
	if code, _, e := (harness{stdin: "s3cret"}).run(t, "add", "-u", "alice", "site"); code != 0 {
		t.Fatalf("add: %d %s", code, e)
	}
	code, stdout, _ := harness{}.run(t, "list", "--json")
	if code != 0 || !strings.Contains(stdout, `"username": "alice"`) {
		t.Errorf("list --json = %d %q, want the username", code, stdout)
	}
}

func TestAddRejectsControlCharactersInUsername(t *testing.T) {
	isolate(t)
	code, _, stderr := harness{stdin: "s3cret"}.run(t, "add", "-u", "bad\x1bname", "site")
	if code != 2 {
		t.Errorf("code %d (stderr %q), want a usage error", code, stderr)
	}
	if metas, _ := store.LoadMetadata(); len(metas) != 0 {
		t.Errorf("a secret with a control character in its username was stored: %+v", metas)
	}
}

func TestUsernameSurvivesExportAndImport(t *testing.T) {
	isolate(t)
	if code, _, e := (harness{stdin: "s3cret"}).run(t, "add", "--username", "bob@example.test", "site"); code != 0 {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "backup.age")
	if code, _, e := withPass(pass).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatal(e)
	}
	if err := store.DeleteSecret("site"); err != nil {
		t.Fatal(err)
	}
	if code, _, e := withPass(pass).run(t, "import", file, "--values"); code != 0 {
		t.Fatal(e)
	}
	metas, _ := store.LoadMetadata()
	if len(metas) != 1 || metas[0].Username != "bob@example.test" {
		t.Errorf("metadata after the round trip = %+v, want the username kept", metas)
	}
}
