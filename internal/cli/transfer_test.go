package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/store"
)

const (
	valueA = "topsecret-1"
	valueB = "p@ss w0rd/2"
	pass   = "correct horse"
)

func seed(t *testing.T) {
	t.Helper()
	mustSave(t, "alpha", "Tokens", valueA)
	mustSave(t, "bravo", "Infra", valueB)
}

func withPass(p string) harness { return harness{env: map[string]string{passphraseEnv: p}} }

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestExportWithoutValuesContainsNoValues(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "meta.json")

	if code, _, e := (harness{}).run(t, "export", "-o", file); code != 0 {
		t.Fatalf("export: %s", e)
	}
	got := readFile(t, file)
	for _, secret := range []string{valueA, valueB, `"value"`} {
		if strings.Contains(got, secret) {
			t.Errorf("metadata export contains %q:\n%s", secret, got)
		}
	}
	for _, want := range []string{`"alpha"`, `"bravo"`, `"values_included": false`, `"expires": "2031-03-04"`} {
		if !strings.Contains(got, want) {
			t.Errorf("metadata export lacks %s:\n%s", want, got)
		}
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(file); info.Mode().Perm() != 0600 {
			t.Errorf("mode = %v, want 0600", info.Mode().Perm())
		}
	}

	// To stdout the same document is produced.
	code, stdout, _ := harness{}.run(t, "export")
	if code != 0 || strings.Contains(stdout, valueA) || !strings.Contains(stdout, `"alpha"`) {
		t.Errorf("stdout export: %d %q", code, stdout)
	}
}

func TestExportWithValuesIsEncryptedAndNeedsPassphrase(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "backup.age")

	// No passphrase source on a non-terminal: refuse, write nothing.
	if code, stdout, stderr := (harness{}).run(t, "export", "--values", "-o", file); code != 1 || !strings.Contains(stderr, passphraseEnv) || stdout != "" {
		t.Errorf("no passphrase: code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatal("a file was written without a passphrase")
	}
	// ... and also when the value would go to stdout.
	if code, stdout, _ := (harness{}).run(t, "export", "--values"); code != 1 || stdout != "" {
		t.Errorf("stdout without passphrase: code %d stdout %q", code, stdout)
	}

	if code, _, e := withPass(pass).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatalf("export --values: %s", e)
	}
	got := readFile(t, file)
	if !strings.HasPrefix(got, armorHeader) {
		t.Errorf("output is not an armored age file: %.60q", got)
	}
	for _, leak := range []string{valueA, valueB, "alpha", "bravo", pass} {
		if strings.Contains(got, leak) {
			t.Errorf("encrypted export leaks %q", leak)
		}
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(file); info.Mode().Perm() != 0600 {
			t.Errorf("mode = %v, want 0600", info.Mode().Perm())
		}
	}

	// Stdout output is encrypted too.
	code, stdout, _ := withPass(pass).run(t, "export", "--values")
	if code != 0 || strings.Contains(stdout, valueA) || !strings.HasPrefix(stdout, armorHeader) {
		t.Errorf("stdout export: %d %.60q", code, stdout)
	}
}

func TestExportPromptsTwiceOnTerminalAndRejectsMismatch(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "b.age")

	code, _, stderr := harness{tty: true, hidden: []string{"one", "two"}}.run(t, "export", "--values", "-o", file)
	if code != 1 || !strings.Contains(stderr, "do not match") {
		t.Errorf("mismatch: %d %q", code, stderr)
	}
	if _, err := os.Stat(file); err == nil {
		t.Error("file written despite mismatched passphrases")
	}
	if code, _, e := (harness{tty: true, hidden: []string{"same", "same"}}).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatal(e)
	}
}

func TestExportRefusesToOverwriteWithoutForce(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(file, []byte("precious"), 0644); err != nil {
		t.Fatal(err)
	}

	if code, _, stderr := (harness{}).run(t, "export", "-o", file); code != 1 || !strings.Contains(stderr, "--force") {
		t.Errorf("code %d stderr %q", code, stderr)
	}
	if readFile(t, file) != "precious" {
		t.Error("existing file was modified")
	}
	if code, _, e := (harness{}).run(t, "export", "-o", file, "--force"); code != 0 {
		t.Fatal(e)
	}
	if !strings.Contains(readFile(t, file), `"alpha"`) {
		t.Error("--force did not replace the file")
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(file); info.Mode().Perm() != 0600 {
			t.Errorf("replaced file mode = %v, want 0600", info.Mode().Perm())
		}
	}
}

func TestExportWithValuesFailsListingEntriesWithoutValue(t *testing.T) {
	isolate(t)
	seed(t)
	if err := keyring.Delete(store.ServiceName, "bravo"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "b.age")
	code, _, stderr := withPass(pass).run(t, "export", "--values", "-o", file)
	if code != 1 || !strings.Contains(stderr, "bravo") || !strings.Contains(stderr, "doctor") {
		t.Errorf("code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(file); err == nil {
		t.Error("partial export written")
	}
}

func TestEncryptedExportImportRoundTrip(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "backup.age")
	if code, _, e := withPass(pass).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatal(e)
	}
	before, _ := store.LoadMetadata()

	for _, title := range []string{"alpha", "bravo"} {
		if err := store.DeleteSecret(title); err != nil {
			t.Fatal(err)
		}
	}

	code, stdout, stderr := withPass(pass).run(t, "import", file, "--values")
	if code != 0 || !strings.Contains(stdout, "Imported 2") {
		t.Fatalf("import: %d %q %q", code, stdout, stderr)
	}
	after, _ := store.LoadMetadata()
	if len(after) != 2 || after[0] != before[0] || after[1] != before[1] {
		t.Errorf("metadata after = %+v, want %+v (dates and notes preserved)", after, before)
	}
	for title, want := range map[string]string{"alpha": valueA, "bravo": valueB} {
		if got, err := store.GetSecretValue(title); err != nil || got != want {
			t.Errorf("value %s = %q, %v; want %q", title, got, err, want)
		}
	}
	if missing, _ := store.MissingValues(); len(missing) != 0 {
		t.Errorf("missing values after restore: %v", missing)
	}
}

func TestImportWithWrongPassphraseFailsAndChangesNothing(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "backup.age")
	if code, _, e := withPass(pass).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatal(e)
	}
	store.DeleteSecret("alpha")
	store.DeleteSecret("bravo")

	code, stdout, stderr := withPass("wrong").run(t, "import", file, "--values")
	if code != 1 || !strings.Contains(stderr, "wrong passphrase") || stdout != "" {
		t.Errorf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if got := titlesOf(t); len(got) != 0 {
		t.Errorf("failed import wrote %v", got)
	}

	if code, _, stderr := (harness{}).run(t, "import", file, "--values"); code != 1 || !strings.Contains(stderr, passphraseEnv) {
		t.Errorf("no passphrase: %d %q", code, stderr)
	}
}

func TestImportRejectsCorruptEncryptedFile(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "backup.age")
	if code, _, e := withPass(pass).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatal(e)
	}
	store.DeleteSecret("alpha")
	store.DeleteSecret("bravo")

	lines := strings.Split(readFile(t, file), "\n")
	mid := len(lines) / 2
	lines[mid] = strings.Map(func(r rune) rune {
		if r == 'A' {
			return 'B'
		}
		return 'A'
	}, lines[mid])
	damaged := filepath.Join(t.TempDir(), "damaged.age")
	if err := os.WriteFile(damaged, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := withPass(pass).run(t, "import", damaged, "--values"); code != 1 {
		t.Errorf("damaged file imported: exit %d", code)
	}
	if got := titlesOf(t); len(got) != 0 {
		t.Errorf("damaged import wrote %v", got)
	}
}

func TestImportSkipsExistingUnlessOverwrite(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "backup.age")
	if code, _, e := withPass(pass).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatal(e)
	}
	// Local state diverges: alpha changed, bravo gone.
	newVal := "changed-locally"
	if err := store.UpdateSecret("alpha", store.SecretMeta{Title: "alpha", Category: "Local"}, &newVal); err != nil {
		t.Fatal(err)
	}
	store.DeleteSecret("bravo")

	code, stdout, stderr := withPass(pass).run(t, "import", file, "--values")
	if code != 0 || !strings.Contains(stdout, "Imported 1, replaced 0, skipped 1") {
		t.Fatalf("import: %d %q %q", code, stdout, stderr)
	}
	if v, _ := store.GetSecretValue("alpha"); v != newVal {
		t.Errorf("skipped secret was overwritten: %q", v)
	}
	if v, _ := store.GetSecretValue("bravo"); v != valueB {
		t.Errorf("bravo = %q", v)
	}

	code, stdout, _ = withPass(pass).run(t, "import", file, "--values", "--overwrite")
	if code != 0 || !strings.Contains(stdout, "Imported 0, replaced 2, skipped 0") {
		t.Fatalf("overwrite import: %d %q", code, stdout)
	}
	if v, _ := store.GetSecretValue("alpha"); v != valueA {
		t.Errorf("alpha after overwrite = %q, want %q", v, valueA)
	}
	if items, _ := store.LoadMetadata(); len(items) != 2 || items[0].Category != "Tokens" {
		t.Errorf("metadata not replaced: %+v", items)
	}
}

func TestImportMetadataOnlyLeavesValuesAbsent(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "meta.json")
	if code, _, e := (harness{}).run(t, "export", "-o", file); code != 0 {
		t.Fatal(e)
	}
	store.DeleteSecret("alpha")
	store.DeleteSecret("bravo")

	code, stdout, _ := harness{}.run(t, "import", file)
	if code != 0 || !strings.Contains(stdout, "Imported 2") || !strings.Contains(stdout, "no value") {
		t.Fatalf("import: %d %q", code, stdout)
	}
	if missing, _ := store.MissingValues(); len(missing) != 2 {
		t.Errorf("missing = %v, want both entries flagged", missing)
	}

	// --values makes no sense for a file without values.
	if code, _, stderr := (harness{}).run(t, "import", file, "--values", "--overwrite"); code != 1 || !strings.Contains(stderr, "no values") {
		t.Errorf("--values on metadata-only file: %d %q", code, stderr)
	}
}

func TestImportWithoutValuesFlagIgnoresValuesInFile(t *testing.T) {
	isolate(t)
	seed(t)
	file := filepath.Join(t.TempDir(), "backup.age")
	if code, _, e := withPass(pass).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatal(e)
	}
	store.DeleteSecret("alpha")
	store.DeleteSecret("bravo")

	code, _, stderr := withPass(pass).run(t, "import", file)
	if code != 0 || !strings.Contains(stderr, "--values") {
		t.Fatalf("import: %d %q", code, stderr)
	}
	if _, err := store.GetSecretValue("alpha"); err == nil {
		t.Error("values were restored without --values")
	}
}

func TestImportValidatesWholeFileBeforeWriting(t *testing.T) {
	isolate(t)
	for name, doc := range map[string]string{
		"bad date": `{"format":"pandora-export","version":1,"secrets":[
			{"title":"good","created":"2026-01-02"},{"title":"bad","created":"2026-01-02","expires":"soon"}]}`,
		"duplicate":    `{"format":"pandora-export","version":1,"secrets":[{"title":"x"},{"title":"x"}]}`,
		"empty title":  `{"format":"pandora-export","version":1,"secrets":[{"title":"ok"},{"title":" "}]}`,
		"wrong format": `{"format":"other","version":1,"secrets":[{"title":"ok"}]}`,
		"new version":  `{"format":"pandora-export","version":99,"secrets":[{"title":"ok"}]}`,
		"not json":     `hello`,
	} {
		file := filepath.Join(t.TempDir(), "in.json")
		if err := os.WriteFile(file, []byte(doc), 0600); err != nil {
			t.Fatal(err)
		}
		if code, _, _ := (harness{}).run(t, "import", file); code != 1 {
			t.Errorf("%s: exit %d, want 1", name, code)
		}
		if got := titlesOf(t); len(got) != 0 {
			t.Errorf("%s: invalid file partially imported: %v", name, got)
		}
	}
}

func TestImportReadsFromStdinAndMissingFileFails(t *testing.T) {
	isolate(t)
	doc := `{"format":"pandora-export","version":1,"secrets":[{"title":"fromstdin","created":"2026-02-03"}]}`
	if code, _, e := (harness{stdin: doc}).run(t, "import", "-"); code != 0 {
		t.Fatal(e)
	}
	if got := titlesOf(t); len(got) != 1 || got[0] != "fromstdin" {
		t.Errorf("titles = %v", got)
	}
	if code, _, _ := (harness{}).run(t, "import", filepath.Join(t.TempDir(), "absent")); code != 1 {
		t.Errorf("missing file: exit %d", code)
	}
}
