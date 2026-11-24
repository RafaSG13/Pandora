package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// addTagged adds a secret through the form, typing tags into the last field.
func addTagged(m tea.Model, name, tags string) tea.Model {
	return press(m, "n", name, "enter", "X", "enter", "enter", "v", "tab", "enter", "enter", tags, "enter")
}

func tagFixture(t *testing.T) tea.Model {
	t.Helper()
	m := addTagged(newTestModel(t), "a", "prod, db")
	m = addTagged(m, "b", "prod")
	return add(m, "c", "X", "v") // no tags
}

func TestTagKeyCyclesThroughTagsThenBackToAll(t *testing.T) {
	m := tagFixture(t)
	for _, step := range []struct {
		tag  string
		want []string
	}{
		{"db", []string{"a"}},
		{"prod", []string{"a", "b"}},
		{"", []string{"a", "b", "c"}},
		{"db", []string{"a"}}, // and round again
	} {
		m = press(m, "T")
		got := visibleTitles(m)
		if m.(model).tag != step.tag || !sameStrings(got, step.want) {
			t.Fatalf("after g: tag=%q shows %v, want tag=%q showing %v", m.(model).tag, got, step.tag, step.want)
		}
	}
	if title := m.(model).list.Title; !strings.Contains(title, "#db") {
		t.Errorf("list title %q does not name the active tag", title)
	}
}

func TestTagFilterCombinesWithCategoryFilter(t *testing.T) {
	m := addTagged(newTestModel(t), "a", "prod")
	m = press(m, "n", "b", "enter", "Y", "enter", "enter", "v", "tab", "enter", "enter", "prod", "enter")
	m = press(m, "T", "1") // tag prod, then the first category (X)
	if got := visibleTitles(m); !sameStrings(got, []string{"a"}) {
		t.Errorf("tag prod + category X shows %v, want [a]", got)
	}
}

func TestTagFilterResetsWhenItsLastSecretIsGone(t *testing.T) {
	m := tagFixture(t)
	m = press(m, "T") // tag db: only a
	m = press(m, "d", "y")
	if mm := m.(model); mm.tag != "" || !sameStrings(visibleTitles(m), []string{"b", "c"}) {
		t.Errorf("after deleting the last db secret: tag=%q shows %v, want all secrets", mm.tag, visibleTitles(m))
	}
}

func TestTagKeyWithoutTagsSaysSo(t *testing.T) {
	m := press(add(newTestModel(t), "a", "X", "v"), "T")
	if mm := m.(model); mm.tag != "" || !strings.Contains(mm.statusMsg, "No tags") {
		t.Errorf("tag=%q status=%q, want a notice that there are no tags", mm.tag, mm.statusMsg)
	}
}

func TestInvalidTagKeepsFormOpenAndSavesNothing(t *testing.T) {
	m := addTagged(newTestModel(t), "a", "two words")
	mm := m.(model)
	if mm.mode != modeForm || !mm.statusErr || !strings.Contains(mm.statusMsg, "Tags") {
		t.Errorf("mode=%v status=%q, want an open form with a Tags error", mm.mode, mm.statusMsg)
	}
	if got := titles(m); len(got) != 0 {
		t.Errorf("a rejected form saved %v", got)
	}
}

func TestDetailCardShowsTags(t *testing.T) {
	m := addTagged(newTestModel(t), "a", "prod, db")
	if out := ansi.Strip(m.View()); !strings.Contains(out, "#db #prod") {
		t.Errorf("detail card does not list the tags:\n%s", out)
	}
}

func TestLockDropsTheTagFilter(t *testing.T) {
	m := press(tagFixture(t), "T")
	mm := m.(model)
	mm.lock()
	if mm.tag != "" || mm.tags != nil {
		t.Errorf("lock left tag=%q tags=%q", mm.tag, mm.tags)
	}
}

// g and G are the list's own "first row" and "last row" keys; the tag filter
// must not shadow them.
func TestGAndShiftGStillJumpToFirstAndLastRow(t *testing.T) {
	m := addThree(newTestModel(t))
	if got := m.(model).list.Index(); got != 2 {
		t.Fatalf("setup: selected row %d, want the last (2)", got)
	}
	m = press(m, "g")
	if got := m.(model).list.Index(); got != 0 {
		t.Errorf("g selected row %d, want the first", got)
	}
	m = press(m, "G")
	if got := m.(model).list.Index(); got != 2 {
		t.Errorf("G selected row %d, want the last", got)
	}
}
