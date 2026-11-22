package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/RafaSG13/Pandora/internal/store"
)

// tempFile writes content to a new file in a temp dir and returns its path.
func tempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func titlesInStore(t *testing.T) []string {
	t.Helper()
	items, err := store.LoadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, m := range items {
		titles = append(titles, m.Title)
	}
	return titles
}

func TestParseDotenvAcceptedSyntax(t *testing.T) {
	src := "\ufeff# a comment\r\n" +
		"\n" +
		"PLAIN=abc\r\n" +
		"export EXPORTED = spaced  \n" +
		"TRAILING=value # a comment\n" +
		"HASH=a#b\n" +
		"SINGLE='no $expansion\\n here'\n" +
		`DOUBLE="tab\there \"quoted\" \$HOME back\\slash \q"` + "\n" +
		"MULTI=\"line one\nline two\"   # after the quote\n" +
		"EMPTY=\n" +
		"EMPTYQUOTES=\"\"\n" +
		"EQUALS=a=b=c\n" +
		"UNICODE=pässwörd-✓\n" +
		"LAST=end" // no trailing newline
	batch, err := parseDotenv([]byte(src), importOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range batch.secrets {
		got[s.Title] = *s.Value
	}
	want := map[string]string{
		"PLAIN":    "abc",
		"EXPORTED": "spaced",
		"TRAILING": "value",
		"HASH":     "a#b",
		"SINGLE":   `no $expansion\n here`,
		"DOUBLE":   "tab\there \"quoted\" $HOME back\\slash \\q",
		"MULTI":    "line one\nline two",
		"EQUALS":   "a=b=c",
		"UNICODE":  "pässwörd-✓",
		"LAST":     "end",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsed %q\nwant   %q", got, want)
	}
	if len(batch.skipped) != 2 || !strings.HasPrefix(batch.skipped[0], "EMPTY:") || !strings.HasPrefix(batch.skipped[1], "EMPTYQUOTES:") {
		t.Errorf("skipped = %q, want the two empty values", batch.skipped)
	}
}

func TestParseDotenvSyntaxErrorsNameTheLineNotTheContent(t *testing.T) {
	const leak = "TOPSECRETVALUE"
	tests := []struct {
		name, src, want string
	}{
		{"unterminated double quote", "A=ok\nB=\"" + leak + "\nC=d\n", "line 2: unterminated quote"},
		{"unterminated single quote", "B='" + leak + "\n", "line 1: unterminated quote"},
		{"text after the closing quote", "A=ok\nB=\"x\" " + leak + "\n", "line 2: unexpected text"},
		{"no equals sign", "A=ok\n" + leak + "\n", "line 2: expected KEY=value"},
		{"bad variable name", "A=ok\nBAD NAME=" + leak + "\n", "line 2: not a usable variable name"},
		{"empty variable name", "=" + leak + "\n", "line 1: not a usable variable name"},
		{"duplicate key", "A=1\n\nA=" + leak + "\n", "line 3: A is already defined on line 1"},
		{"line count after a multi-line value", "A=\"x\ny\nz\"\nB " + leak + "\n", "line 4: expected KEY=value"},
	}
	for _, tc := range tests {
		_, err := parseDotenv([]byte(tc.src), importOptions{})
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), leak) {
			t.Errorf("%s: error %v, want it to contain %q and never the content", tc.name, err, tc.want)
		}
	}
	if _, err := parseDotenv([]byte("A=\xff\xfe\n"), importOptions{}); err == nil {
		t.Error("invalid UTF-8 was accepted")
	}
}

func TestParseDotenvSkipsUnusableEntriesAndKeepsTheRest(t *testing.T) {
	big := strings.Repeat("x", store.MaxValueBytes+1)
	batch, err := parseDotenv([]byte("BIG="+big+"\nOK=fine\n"), importOptions{category: "ci"})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.secrets) != 1 || batch.secrets[0].Title != "OK" || batch.secrets[0].Category != "ci" {
		t.Errorf("secrets = %+v, want only OK in category ci", batch.secrets)
	}
	if len(batch.skipped) != 1 || !strings.HasPrefix(batch.skipped[0], `"BIG":`) || strings.Contains(batch.skipped[0], "xxxx") {
		t.Errorf("skipped = %q, want BIG without its value", batch.skipped)
	}
}

// What `env` prints must come back through `import --format dotenv` unchanged.
func TestEnvOutputRoundTripsThroughDotenvImport(t *testing.T) {
	isolate(t)
	values := map[string]string{
		"PLAIN":  "abc123",
		"SPACES": "  leading and trailing  ",
		"QUOTES": `say "hi" and 'bye'`,
		"DOLLAR": "$HOME ${X} $(cmd) `tick`",
		"SLASH":  `C:\dir\new\table\`,
		"MULTI":  "-----BEGIN KEY-----\nline2\r\nline3\n",
		"HASH":   "a #b # c",
		"EQ":     "k=v=w",
		"UNI":    "pässwörd ✓",
	}
	var args []string
	for k, v := range values {
		if err := store.SaveSecret(store.SecretMeta{Title: k, Created: "2026-01-01"}, v); err != nil {
			t.Fatal(err)
		}
		args = append(args, "-s", k)
	}
	code, dotenv, stderr := harness{}.run(t, append([]string{"env"}, args...)...)
	if code != 0 {
		t.Fatalf("env: exit %d: %s", code, stderr)
	}
	for k := range values {
		if err := store.DeleteSecret(k); err != nil {
			t.Fatal(err)
		}
	}

	code, stdout, stderr := harness{stdin: dotenv}.run(t, "import", "-", "--format", "dotenv")
	if code != 0 || !strings.Contains(stdout, "Imported 9, replaced 0, skipped 0") {
		t.Fatalf("import: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	for k, want := range values {
		if got, err := store.GetSecretValue(k); err != nil || got != want {
			t.Errorf("%s = %q (%v), want %q", k, got, err, want)
		}
	}
}

func TestImportDotenvFileCategoryAndPlainTextReminder(t *testing.T) {
	isolate(t)
	file := tempFile(t, "A=1\nB=2\n")
	code, stdout, stderr := harness{}.run(t, "import", file, "--format", "dotenv", "--category", "ci")
	if code != 0 || !strings.Contains(stdout, "Imported 2") || !strings.Contains(stderr, "plain text") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	items, _ := store.LoadMetadata()
	for _, m := range items {
		if m.Category != "ci" || m.Created == "" {
			t.Errorf("%+v: want category ci and a creation date", m)
		}
	}
}

func TestImportDotenvSyntaxErrorWritesNothing(t *testing.T) {
	isolate(t)
	code, stdout, stderr := harness{stdin: "A=1\nB=\"unterminated\n"}.run(t, "import", "-", "--format", "dotenv")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "line 2") || len(titlesInStore(t)) != 0 {
		t.Errorf("exit %d, stdout %q, stderr %q, stored %v; want exit 1 and nothing imported", code, stdout, stderr, titlesInStore(t))
	}
}

func TestImportDryRunWritesNothingAndFollowsOverwrite(t *testing.T) {
	isolate(t)
	if err := store.SaveSecret(store.SecretMeta{Title: "OLD", Created: "2026-01-01"}, "keep"); err != nil {
		t.Fatal(err)
	}
	src := "OLD=new\nFRESH=1\n"
	code, stdout, stderr := harness{stdin: src}.run(t, "import", "-", "--format", "dotenv", "--dry-run")
	if code != 0 || !strings.Contains(stdout, "skip     OLD") || !strings.Contains(stdout, "import   FRESH") || !strings.Contains(stdout, "Would import 1, replace 0, skip 1") {
		t.Fatalf("dry run: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, _ = harness{stdin: src}.run(t, "import", "-", "--format", "dotenv", "--dry-run", "--overwrite")
	if code != 0 || !strings.Contains(stdout, "replace  OLD") || !strings.Contains(stdout, "Would import 1, replace 1, skip 0") {
		t.Fatalf("dry run --overwrite: exit %d, stdout %q", code, stdout)
	}
	if got := titlesInStore(t); !reflect.DeepEqual(got, []string{"OLD"}) {
		t.Errorf("dry run changed the vault: %v", got)
	}
	if v, _ := store.GetSecretValue("OLD"); v != "keep" {
		t.Errorf("dry run changed OLD to %q", v)
	}
}

func TestImportDryRunAlsoWorksForPandoraExports(t *testing.T) {
	isolate(t)
	mustSave(t, "alpha", "c", "v")
	code, export, _ := harness{}.run(t, "export")
	if code != 0 {
		t.Fatal("export failed")
	}
	if err := store.DeleteSecret("alpha"); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := harness{stdin: export}.run(t, "import", "-", "--dry-run")
	if code != 0 || !strings.Contains(stdout, "import   alpha (no value)") || len(titlesInStore(t)) != 0 {
		t.Errorf("exit %d, stdout %q, stored %v", code, stdout, titlesInStore(t))
	}
}

func TestImportRejectsInconsistentFormatFlags(t *testing.T) {
	isolate(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown format", []string{"--format", "lastpass"}, "dotenv, bitwarden-json"},
		{"values with a foreign format", []string{"--format", "dotenv", "--values"}, "--values only applies"},
		{"category with the pandora format", []string{"--category", "x"}, "--category only applies"},
		{"category with bitwarden", []string{"--format", "bitwarden-json", "--category", "x"}, "--category only applies"},
		{"category with a control character", []string{"--format", "dotenv", "--category", "a\x1bb"}, "invalid --category"},
	}
	for _, tc := range tests {
		code, stdout, stderr := harness{stdin: "A=1\n"}.run(t, append([]string{"import", "-"}, tc.args...)...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, tc.want) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want exit 2 mentioning %q", tc.name, code, stdout, stderr, tc.want)
		}
	}
}

func TestImportForeignFormatRefusesEncryptedFile(t *testing.T) {
	isolate(t)
	mustSave(t, "alpha", "c", "v")
	file := t.TempDir() + "/x.age"
	env := map[string]string{passphraseEnv: "a long enough passphrase"}
	if code, _, e := (harness{env: env}).run(t, "export", "--values", "-o", file); code != 0 {
		t.Fatal(e)
	}
	code, stdout, stderr := harness{env: env}.run(t, "import", file, "--format", "dotenv")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "age-encrypted") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// A synthetic export that follows Bitwarden's documented JSON layout; no real data.
const bitwardenFixture = `{
  "encrypted": false,
  "folders": [{"id": "f1", "name": "Work/Servers"}, {"id": "f2", "name": " Personal "}],
  "items": [
    {"id": "1", "folderId": "f1", "type": 1, "name": "  Prod DB  ", "notes": "line one\nline two\t\u001b[31mred",
     "creationDate": "2024-02-03T10:11:12.000Z", "revisionDate": "2025-06-07T01:02:03.000Z",
     "login": {"username": "admin", "password": "p@ss\nword", "totp": "otpauth://totp/x?secret=ABC",
               "uris": [{"uri": ""}, {"uri": "https://db.example.test"}, {"uri": "https://second.example.test"}]}},
    {"id": "2", "folderId": null, "type": 1, "name": "Prod DB", "notes": null,
     "login": {"username": null, "password": "second", "totp": null, "uris": []}},
    {"id": "3", "folderId": "f2", "type": 2, "name": "Recovery codes", "notes": "code-1\ncode-2\n", "login": null},
    {"id": "4", "type": 3, "name": "Visa", "card": {"number": "4111"}},
    {"id": "5", "type": 4, "name": "Me", "identity": {"firstName": "A"}},
    {"id": "6", "type": 1, "name": "No password", "login": {"username": "u", "password": ""}},
    {"id": "7", "type": 1, "name": "Prod DB", "login": {"password": "third"}},
    {"id": "8", "type": 2, "name": "Blank note", "notes": "  \n"},
    {"id": "9", "type": 1, "name": "", "login": {"password": "nameless"}},
    {"id": "10", "type": 9, "name": "Future kind"},
    {"id": "11", "type": 1, "name": "Bad\u0007name", "login": {"password": "x"}},
    {"id": "12", "folderId": "missing", "type": 1, "name": "Orphan folder", "login": {"password": "o", "uris": []},
     "creationDate": "garbage"}
  ]
}`

func TestParseBitwardenJSONMapping(t *testing.T) {
	batch, err := parseBitwardenJSON([]byte(bitwardenFixture), importOptions{})
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ title, category, notes, value, created, updated string }
	var got []row
	for _, s := range batch.secrets {
		got = append(got, row{s.Title, s.Category, s.Notes, *s.Value, s.Created, s.Updated})
	}
	today := batch.secrets[len(batch.secrets)-1].Created // dates that were not usable fall back to today
	want := []row{
		{"Prod DB", "Work/Servers", "line one line two [31mred | user: admin | url: https://db.example.test", "p@ss\nword", "2024-02-03", "2025-06-07"},
		{"Prod DB (2)", "", "", "second", today, ""},
		{"Recovery codes", "Personal", "", "code-1\ncode-2\n", today, ""},
		{"Prod DB (3)", "", "", "third", today, ""},
		{"Orphan folder", "", "", "o", today, ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("secrets:\n got %q\nwant %q", got, want)
	}

	wantSkipped := []string{`"Visa": cards`, `"Me": identities`, `"No password": login without`, `"Blank note": empty secure note`, "item #9: empty title", `"Future kind": unsupported item type 9`, "control characters"}
	if len(batch.skipped) != len(wantSkipped) {
		t.Fatalf("skipped %q, want %d entries", batch.skipped, len(wantSkipped))
	}
	for i, w := range wantSkipped {
		if !strings.Contains(batch.skipped[i], w) {
			t.Errorf("skipped[%d] = %q, want it to contain %q", i, batch.skipped[i], w)
		}
	}
	notes := strings.Join(batch.notes, "\n")
	if !strings.Contains(notes, `"Prod DB": TOTP seed not imported`) || !strings.Contains(notes, `imported as "Prod DB (2)"`) || !strings.Contains(notes, `imported as "Prod DB (3)"`) {
		t.Errorf("notes = %q", batch.notes)
	}
	for _, s := range append(append([]string{}, batch.skipped...), batch.notes...) {
		for _, secret := range []string{"p@ss", "second", "third", "nameless", "ABC", "code-1"} {
			if strings.Contains(s, secret) {
				t.Errorf("report line %q leaks %q", s, secret)
			}
		}
	}
}

func TestParseBitwardenJSONRejectsWrongFiles(t *testing.T) {
	tests := map[string]string{
		"encrypted export": `{"encrypted": true, "encKeyValidation_DO_NOT_EDIT": "2.x", "data": "2.y"}`,
		"not json":         `A=1`,
		"no items":         `{"folders": []}`,
		"a pandora export": `{"format":"pandora-export","version":1,"secrets":[]}`,
	}
	for name, src := range tests {
		if _, err := parseBitwardenJSON([]byte(src), importOptions{}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if b, err := parseBitwardenJSON([]byte(`{"items": []}`), importOptions{}); err != nil || len(b.secrets) != 0 {
		t.Errorf("empty export: %v, %v", b, err)
	}
}

func TestImportBitwardenEndToEnd(t *testing.T) {
	isolate(t)
	if err := store.SaveSecret(store.SecretMeta{Title: "Recovery codes", Created: "2026-01-01"}, "mine"); err != nil {
		t.Fatal(err)
	}
	file := tempFile(t, bitwardenFixture)
	code, stdout, stderr := harness{}.run(t, "import", file, "--format", "bitwarden-json")
	if code != 0 || !strings.Contains(stdout, "Imported 4, replaced 0, skipped 1") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	for _, want := range []string{"Not imported (7)", "TOTP seed not imported", "plain text"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if v, _ := store.GetSecretValue("Prod DB"); v != "p@ss\nword" {
		t.Errorf("Prod DB = %q", v)
	}
	if v, _ := store.GetSecretValue("Recovery codes"); v != "mine" {
		t.Errorf("existing secret was replaced without --overwrite: %q", v)
	}
	items, _ := store.LoadMetadata()
	for _, m := range items {
		if m.Title == "Prod DB" && (m.Category != "Work/Servers" || m.Created != "2024-02-03" || m.Updated != "2025-06-07") {
			t.Errorf("Prod DB metadata = %+v", m)
		}
	}
}
