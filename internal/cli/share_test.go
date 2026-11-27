package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/store"
)

// switchVault points the process at another machine's vault, creating it.
func switchVault(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
	store.Lock()
	var err error
	if store.Exists() {
		err = store.Unlock(testMaster)
	} else {
		err = store.Create(testMaster)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func idOf(t *testing.T) string {
	t.Helper()
	code, out, errOut := harness{}.run(t, "id")
	if code != 0 || !strings.HasPrefix(out, store.ShareKeyPrefix) || !strings.Contains(errOut, "Fingerprint:") {
		t.Fatalf("id = %d %q %q", code, out, errOut)
	}
	return strings.TrimSpace(out)
}

func TestShareBetweenTwoMachines(t *testing.T) {
	isolate(t)
	dirA := t.TempDir()
	dirB := t.TempDir()
	tmp := t.TempDir()

	switchVault(t, dirA)
	keyA := idOf(t)
	switchVault(t, dirB)
	keyB := idOf(t)
	fa, fb := filepath.Join(tmp, "a.key"), filepath.Join(tmp, "b.key")
	if err := os.WriteFile(fa, []byte(keyA), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fb, []byte(keyB), 0600); err != nil {
		t.Fatal(err)
	}

	if code, _, _ := (harness{}).run(t, "peer", "add", "desktop", fa); code != 1 {
		t.Errorf("peer add without -y on non-tty = %d, want 1", code)
	}
	if code, _, _ := (harness{}).run(t, "peer", "add", "desktop", "garbage", "-y"); code != 1 {
		t.Errorf("peer add with missing file = %d, want 1", code)
	}
	if code, _, _ := (harness{}).run(t, "peer", "add", "desktop", "pandora-share-1:junk", "-y"); code != 2 {
		t.Errorf("peer add bad key = %d, want 2", code)
	}
	if code, _, e := (harness{}).run(t, "peer", "add", "desktop", fa, "-y"); code != 0 {
		t.Fatalf("peer add: %s", e)
	}
	if code, _, _ := (harness{}).run(t, "peer", "add", "me", fb, "-y"); code != 1 {
		t.Errorf("peer add own key = %d, want 1", code)
	}
	if _, out, _ := (harness{}).run(t, "peer", "list", "--json"); !strings.Contains(out, `"desktop"`) {
		t.Errorf("peer list --json = %q", out)
	}

	switchVault(t, dirA)
	if code, _, e := (harness{}).run(t, "peer", "add", "laptop", fb, "-y"); code != 0 {
		t.Fatalf("peer add: %s", e)
	}
	seed(t)
	out := filepath.Join(tmp, "share.age")
	if code, _, _ := (harness{}).run(t, "export", "--to", "laptop", "-o", out); code != 2 {
		t.Errorf("--to without --values = %d, want 2", code)
	}
	if code, _, _ := (harness{}).run(t, "export", "--values", "--to", "laptop", "-s", "nope", "-o", out); code != 1 {
		t.Errorf("unknown -s = %d, want 1", code)
	}
	if code, _, e := (harness{}).run(t, "export", "--values", "--to", "laptop", "-s", "alpha", "-o", out); code != 0 {
		t.Fatalf("export: %s", e)
	}
	got := readFile(t, out)
	for _, leak := range []string{valueA, "alpha"} {
		if strings.Contains(got, leak) {
			t.Errorf("share file contains %q", leak)
		}
	}

	switchVault(t, dirB)
	code, _, e := harness{}.run(t, "import", out, "--values")
	if code != 0 || !strings.Contains(e, `Shared by peer "desktop"`) {
		t.Fatalf("import = %d %q", code, e)
	}
	if _, v, _ := (harness{}).run(t, "get", "alpha", "-q"); v != valueA {
		t.Errorf("get alpha = %q, want %q", v, valueA)
	}
	if got := titlesOf(t); len(got) != 1 || got[0] != "alpha" {
		t.Errorf("titles in B = %v, want [alpha]", got)
	}

	if code, _, e := (harness{}).run(t, "peer", "rm", "desktop", "-y"); code != 0 {
		t.Fatalf("peer rm: %s", e)
	}
	if code, _, e := (harness{}).run(t, "import", out, "--values", "--overwrite"); code != 1 || !strings.Contains(e, "not a trusted peer") {
		t.Errorf("import after rm = %d %q", code, e)
	}
}
