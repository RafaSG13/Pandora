package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// setupHistory is setup with history enabled to depth.
func setupHistory(t *testing.T, depth int) {
	t.Helper()
	setup(t)
	SetHistoryDepth(depth)
	t.Cleanup(func() { SetHistoryDepth(0) })
}

func update(t *testing.T, title, value string) {
	t.Helper()
	if err := UpdateSecret(title, SecretMeta{Title: title, Category: "X"}, &value); err != nil {
		t.Fatalf("UpdateSecret(%q): %v", title, err)
	}
}

func versionValues(t *testing.T, title string) []string {
	t.Helper()
	infos, err := Versions(title)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for back := 1; back <= len(infos); back++ {
		v, err := GetVersion(title, back)
		if err != nil {
			t.Fatalf("GetVersion(%q, %d): %v", title, back, err)
		}
		out = append(out, v)
	}
	return out
}

func TestUpdateKeepsPreviousValuesUpToTheDepth(t *testing.T) {
	setupHistory(t, 2)
	mustSave(t, "k", "X", "v1")
	first := recOf(t, "k")
	update(t, "k", "v2")
	update(t, "k", "v3")
	update(t, "k", "v4")

	if got, _ := GetSecretValue("k"); got != "v4" {
		t.Errorf("current = %q", got)
	}
	if got := versionValues(t, "k"); !reflect.DeepEqual(got, []string{"v3", "v2"}) {
		t.Errorf("history = %q, want [v3 v2] newest first", got)
	}
	if entryExists(first, 0) {
		t.Error("the value pruned past the depth is still in the keyring")
	}
	infos, _ := Versions("k")
	if len(infos) != 2 || infos[0].Replaced == "" {
		t.Errorf("versions = %+v, want two with a replaced date", infos)
	}
}

func TestUpdateWithHistoryDisabledDropsTheOldValue(t *testing.T) {
	setupHistory(t, 0)
	mustSave(t, "k", "X", "v1")
	first := recOf(t, "k")
	update(t, "k", "v2")
	if entryExists(first, 0) || len(versionValues(t, "k")) != 0 {
		t.Error("history is disabled but the old value was kept")
	}
}

func TestSettingTheSameValueCreatesNoVersion(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "k", "X", "v1")
	id := recOf(t, "k").ID
	update(t, "k", "v1")
	if got := versionValues(t, "k"); len(got) != 0 {
		t.Errorf("history = %q after saving an unchanged value", got)
	}
	if recOf(t, "k").ID != id {
		t.Error("an unchanged value was rewritten under a new keyring entry")
	}
}

func TestMetadataChangesAndRenamesKeepTheHistory(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "old", "X", "v1")
	update(t, "old", "v2")
	if err := UpdateSecret("old", SecretMeta{Title: "new", Category: "Y", Notes: "n"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := versionValues(t, "new"); !reflect.DeepEqual(got, []string{"v1"}) {
		t.Errorf("history after rename = %q", got)
	}
	if _, err := Versions("old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Versions(old) = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemovesEveryVersionIncludingChunks(t *testing.T) {
	setupHistory(t, 3)
	big := pemLike(5000) // several keyring entries
	mustSave(t, "k", "X", big)
	update(t, "k", "v2")
	update(t, "k", "v3")
	r := recOf(t, "k")
	entries := []record{{ID: r.ID, Chunks: r.Chunks}}
	for _, v := range r.History {
		entries = append(entries, v.asRecord())
	}
	if len(entries) != 3 || entries[2].chunkCount() < 3 {
		t.Fatalf("setup: entries %+v", entries)
	}
	if err := DeleteSecret("k"); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		for k := range e.chunkCount() {
			if entryExists(e, k) {
				t.Errorf("entry %s chunk %d survived the delete", e.ID, k)
			}
		}
	}
}

func TestLongValuesRestoreFromHistoryIntact(t *testing.T) {
	setupHistory(t, 2)
	big := pemLike(7000)
	mustSave(t, "k", "X", big)
	update(t, "k", "short")
	if got, err := GetVersion("k", 1); err != nil || got != big {
		t.Fatalf("previous value damaged: len %d, %v", len(got), err)
	}
	if err := RestoreVersion("k", 1); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetSecretValue("k"); got != big {
		t.Error("restored long value differs")
	}
	if got := versionValues(t, "k"); !reflect.DeepEqual(got, []string{"short"}) {
		t.Errorf("history after restore = %q", got)
	}
}

func TestRestoreVersionSwapsAndTouchesOnlyTheVault(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "k", "X", "v1")
	update(t, "k", "v2")
	update(t, "k", "v3")
	before := recOf(t, "k")
	ids := func(r record) []string {
		out := []string{r.ID}
		for _, v := range r.History {
			out = append(out, v.ID)
		}
		return out
	}
	allBefore := ids(before)

	if err := RestoreVersion("k", 2); err != nil { // v1
		t.Fatal(err)
	}
	if got, _ := GetSecretValue("k"); got != "v1" {
		t.Errorf("current = %q, want v1", got)
	}
	if got := versionValues(t, "k"); !reflect.DeepEqual(got, []string{"v3", "v2"}) {
		t.Errorf("history = %q, want [v3 v2]: the replaced value goes on top", got)
	}
	after := ids(recOf(t, "k"))
	if len(after) != len(allBefore) {
		t.Fatalf("entry count changed: %v -> %v", allBefore, after)
	}
	for _, id := range after {
		found := false
		for _, b := range allBefore {
			found = found || id == b
		}
		if !found {
			t.Errorf("restore created a keyring entry %s", id)
		}
	}

	// The restore is itself undoable.
	if err := RestoreVersion("k", 1); err != nil { // v3
		t.Fatal(err)
	}
	if got, _ := GetSecretValue("k"); got != "v3" {
		t.Errorf("undo: current = %q, want v3", got)
	}
}

func TestRestoreVersionRefusesWithoutChangingAnything(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "k", "X", "v1")
	update(t, "k", "v2")
	p, _ := vaultPath()
	snapshot := func() []byte { b, _ := os.ReadFile(p); return b }
	before := snapshot()

	for _, back := range []int{0, -1, 2, 99} {
		if err := RestoreVersion("k", back); !errors.Is(err, ErrNoSuchVersion) {
			t.Errorf("back %d: err = %v, want ErrNoSuchVersion", back, err)
		}
		if _, err := GetVersion("k", back); !errors.Is(err, ErrNoSuchVersion) {
			t.Errorf("GetVersion back %d: err = %v", back, err)
		}
	}
	if err := RestoreVersion("nope", 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown title: %v", err)
	}

	prev := recOf(t, "k").History[0]
	if err := keyring.Delete(ServiceName, prev.ID); err != nil {
		t.Fatal(err)
	}
	if err := RestoreVersion("k", 1); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("missing value: err = %v, want keyring.ErrNotFound", err)
	}
	if !bytes.Equal(snapshot(), before) {
		t.Error("a refused restore rewrote the vault")
	}
}

func TestLoweringTheDepthPrunesOnTheNextChange(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "k", "X", "v1")
	update(t, "k", "v2")
	update(t, "k", "v3")
	update(t, "k", "v4")
	r := recOf(t, "k")
	oldest := r.History[2]

	SetHistoryDepth(1)
	update(t, "k", "v5")
	if got := versionValues(t, "k"); !reflect.DeepEqual(got, []string{"v4"}) {
		t.Errorf("history = %q, want [v4]", got)
	}
	if entryExists(oldest.asRecord(), 0) {
		t.Error("a pruned version is still in the keyring")
	}
}

func TestUpdatingARecordWhoseValueIsGoneDoesNotKeepIt(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "k", "X", "v1")
	if err := keyring.Delete(ServiceName, recOf(t, "k").ID); err != nil {
		t.Fatal(err)
	}
	update(t, "k", "v2")
	if got := versionValues(t, "k"); len(got) != 0 {
		t.Errorf("a missing value became a version: %q", got)
	}
	if got, _ := GetSecretValue("k"); got != "v2" {
		t.Errorf("current = %q", got)
	}
}

func TestFailedVaultWriteLeavesTheOldValueAndHistoryUntouched(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "k", "X", "v1")
	update(t, "k", "v2")
	before := recOf(t, "k")
	readOnlyVaultDir(t)

	v3 := "v3"
	if err := UpdateSecret("k", SecretMeta{Title: "k", Category: "X"}, &v3); err == nil {
		t.Fatal("UpdateSecret succeeded with an unwritable vault")
	}
	if got, err := GetSecretValue("k"); err != nil || got != "v2" {
		t.Errorf("current = %q, %v", got, err)
	}
	if got := versionValues(t, "k"); !reflect.DeepEqual(got, []string{"v1"}) {
		t.Errorf("history = %q", got)
	}
	if !entryExists(before, 0) || !entryExists(before.History[0].asRecord(), 0) {
		t.Error("a failed update removed entries the vault still refers to")
	}
}

func TestRestoreSecretsKeepsHistoryAndIgnoresUnchangedValues(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "k", "X", "v1")
	v2, same := "v2", "v2"
	if err := RestoreSecrets([]Restore{{Meta: SecretMeta{Title: "k", Category: "Z"}, Value: &v2}}); err != nil {
		t.Fatal(err)
	}
	if err := RestoreSecrets([]Restore{{Meta: SecretMeta{Title: "k", Category: "Z"}, Value: &same}}); err != nil {
		t.Fatal(err)
	}
	if got := versionValues(t, "k"); !reflect.DeepEqual(got, []string{"v1"}) {
		t.Errorf("history = %q, want exactly [v1]: the unchanged import must not add a version", got)
	}
	if got, _ := GetSecretValue("k"); got != "v2" {
		t.Errorf("current = %q", got)
	}
}

func TestVersionCountsListsOnlySecretsWithHistory(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "a", "X", "1")
	mustSave(t, "b", "X", "1")
	update(t, "b", "2")
	update(t, "b", "3")
	got, err := VersionCounts()
	if err != nil || !reflect.DeepEqual(got, map[string]int{"b": 2}) {
		t.Errorf("VersionCounts = %v, %v", got, err)
	}
}

func TestMissingVersionsAreReportedAndDropped(t *testing.T) {
	setupHistory(t, 3)
	mustSave(t, "k", "X", "v1")
	update(t, "k", "v2")
	update(t, "k", "v3")
	r := recOf(t, "k")
	if err := keyring.Delete(ServiceName, r.History[1].ID); err != nil { // v1
		t.Fatal(err)
	}

	if cur, err := MissingValues(); err != nil || len(cur) != 0 {
		t.Errorf("MissingValues = %v, %v: only previous values are gone", cur, err)
	}
	got, err := MissingVersions()
	if err != nil || !reflect.DeepEqual(got, []MissingVersion{{"k", 2}}) {
		t.Fatalf("MissingVersions = %v, %v", got, err)
	}
	n, err := DropMissingVersions()
	if err != nil || n != 1 {
		t.Fatalf("DropMissingVersions = %d, %v", n, err)
	}
	if got := versionValues(t, "k"); !reflect.DeepEqual(got, []string{"v2"}) {
		t.Errorf("history after cleanup = %q", got)
	}
	if n, _ := DropMissingVersions(); n != 0 {
		t.Errorf("second cleanup removed %d", n)
	}
}

// Old vaults have no history and a record without it must stay byte-compatible.
func TestHistoryFieldIsOmittedWhenEmpty(t *testing.T) {
	raw, err := json.Marshal(record{ID: "abc", SecretMeta: SecretMeta{Title: "t"}})
	if err != nil || strings.Contains(string(raw), "history") {
		t.Errorf("marshalled record %s (%v) must not mention history", raw, err)
	}
	var old record
	if err := json.Unmarshal([]byte(`{"id":"abc","title":"t","category":"c","created":"2026-01-02"}`), &old); err != nil || len(old.History) != 0 {
		t.Errorf("old record: %+v, %v", old, err)
	}
	setupHistory(t, 3)
	mustSave(t, "plain", "X", "v")
	if got, err := Versions("plain"); err != nil || len(got) != 0 {
		t.Errorf("a secret that never changed has versions %v, %v", got, err)
	}
}
