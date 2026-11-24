package store

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got, err := NormalizeTags([]string{" Prod ", "db", "PROD", "", "  ", "Äpfel"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"db", "prod", "äpfel"}; !reflect.DeepEqual(got, want) {
		t.Errorf("NormalizeTags = %q, want %q (trimmed, lowercase, unique, sorted)", got, want)
	}
	if got, err := NormalizeTags([]string{"", " "}); err != nil || got != nil {
		t.Errorf("only blanks = %q, %v; want nil so the field is omitted", got, err)
	}
}

func TestNormalizeTagsRejects(t *testing.T) {
	long := strings.Repeat("é", MaxTagLen+1)
	many := make([]string, MaxTags+1)
	for i := range many {
		many[i] = string(rune('a' + i))
	}
	for name, c := range map[string]struct {
		tags []string
		want error
	}{
		"space inside":  {[]string{"two words"}, ErrInvalidTag},
		"comma inside":  {[]string{"a,b"}, ErrInvalidTag},
		"too long":      {[]string{long}, ErrInvalidTag},
		"too many":      {many, ErrInvalidTag},
		"control":       {[]string{"a\x1b[2J"}, ErrInvalidText},
		"tab inside":    {[]string{"a\tb"}, ErrInvalidText},
		"nbsp is a gap": {[]string{"a\u00a0b"}, ErrInvalidTag},
	} {
		if _, err := NormalizeTags(c.tags); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	// The limits are inclusive, and duplicates do not count twice.
	ok := strings.Repeat("é", MaxTagLen)
	if _, err := NormalizeTags([]string{ok}); err != nil {
		t.Errorf("tag of exactly %d runes: %v", MaxTagLen, err)
	}
	if _, err := NormalizeTags(append(many[:MaxTags:MaxTags], "A", "B")); err != nil {
		t.Errorf("%d tags plus duplicates: %v", MaxTags, err)
	}
}

func TestParseTags(t *testing.T) {
	got, err := ParseTags(" Web, db ,,web")
	if err != nil || !reflect.DeepEqual(got, []string{"db", "web"}) {
		t.Errorf("ParseTags = %q, %v", got, err)
	}
	if got, err := ParseTags(""); err != nil || got != nil {
		t.Errorf("ParseTags(\"\") = %q, %v; want nil", got, err)
	}
	if _, err := ParseTags("a b, c"); !errors.Is(err, ErrInvalidTag) {
		t.Errorf("ParseTags with a space inside a tag: %v", err)
	}
}

func TestTagsAreNormalisedAndPersisted(t *testing.T) {
	setup(t)
	if err := SaveSecret(SecretMeta{Title: "a", Category: "X", Tags: []string{"Prod", "db", "prod"}}, "v"); err != nil {
		t.Fatal(err)
	}
	metas, err := LoadMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"db", "prod"}; !reflect.DeepEqual(metas[0].Tags, want) {
		t.Errorf("stored tags = %q, want %q", metas[0].Tags, want)
	}

	// Update replaces them; renaming keeps them when the caller passes them.
	if err := UpdateSecret("a", SecretMeta{Title: "b", Category: "X", Tags: []string{"new"}}, nil); err != nil {
		t.Fatal(err)
	}
	metas, _ = LoadMetadata()
	if want := []string{"new"}; !reflect.DeepEqual(metas[0].Tags, want) {
		t.Errorf("tags after update = %q, want %q", metas[0].Tags, want)
	}
	if err := UpdateSecret("b", SecretMeta{Title: "b"}, nil); err != nil {
		t.Fatal(err)
	}
	if metas, _ = LoadMetadata(); len(metas[0].Tags) != 0 {
		t.Errorf("tags = %q after saving none, want cleared", metas[0].Tags)
	}
}

func TestInvalidTagsAreRejectedByEveryWriter(t *testing.T) {
	setup(t)
	bad := SecretMeta{Title: "t", Tags: []string{"has space"}}
	if err := SaveSecret(bad, "v"); !errors.Is(err, ErrInvalidTag) {
		t.Errorf("SaveSecret = %v", err)
	}
	mustSave(t, "ok", "X", "v")
	if err := UpdateSecret("ok", bad, nil); !errors.Is(err, ErrInvalidTag) {
		t.Errorf("UpdateSecret = %v", err)
	}
	if err := RestoreSecrets([]Restore{{Meta: bad}}); !errors.Is(err, ErrInvalidTag) {
		t.Errorf("RestoreSecrets = %v", err)
	}
	if got := titles(t); len(got) != 1 {
		t.Errorf("titles = %v, want only [ok]", got)
	}
}

func TestRestoreNormalisesWithoutTouchingTheCallersSlice(t *testing.T) {
	setup(t)
	in := []Restore{{Meta: SecretMeta{Title: "r", Tags: []string{"B", "a"}}}}
	if err := RestoreSecrets(in); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in[0].Meta.Tags, []string{"B", "a"}) {
		t.Errorf("caller's slice was modified: %q", in[0].Meta.Tags)
	}
	metas, _ := LoadMetadata()
	if want := []string{"a", "b"}; !reflect.DeepEqual(metas[0].Tags, want) {
		t.Errorf("restored tags = %q, want %q", metas[0].Tags, want)
	}
}

// A vault written before tags existed has records without the field; it must
// decode with no tags, and a secret without tags must keep writing exactly
// the old shape, so older versions read the file unchanged.
func TestRecordsWithoutTagsKeepTheOldShape(t *testing.T) {
	var r record
	old := `{"id":"abc","title":"t","category":"c","created":"2026-01-02"}`
	if err := json.Unmarshal([]byte(old), &r); err != nil {
		t.Fatal(err)
	}
	if r.Tags != nil || r.Title != "t" {
		t.Fatalf("old record decoded as %+v", r)
	}
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "tags") {
		t.Errorf("record without tags marshals a tags key: %s", out)
	}
}

func TestUsernameRoundTripsAndRejectsControlCharacters(t *testing.T) {
	setup(t)
	if err := SaveSecret(SecretMeta{Title: "site", Username: "alice", Created: "2026-01-02"}, "v"); err != nil {
		t.Fatal(err)
	}
	metas, err := LoadMetadata()
	if err != nil || len(metas) != 1 || metas[0].Username != "alice" {
		t.Fatalf("LoadMetadata = %+v, %v; want the username", metas, err)
	}
	err = SaveSecret(SecretMeta{Title: "evil", Username: "a\x1b[31m", Created: "2026-01-02"}, "v")
	if !errors.Is(err, ErrInvalidText) {
		t.Errorf("SaveSecret with an escape in the username = %v, want ErrInvalidText", err)
	}
	v := "v2"
	if err := UpdateSecret("site", SecretMeta{Title: "site", Username: "bob"}, &v); err != nil {
		t.Fatal(err)
	}
	if metas, _ := LoadMetadata(); metas[0].Username != "bob" {
		t.Errorf("username after UpdateSecret = %q, want bob", metas[0].Username)
	}
}
