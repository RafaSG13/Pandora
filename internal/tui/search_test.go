package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// runCmd runs cmd (and the commands of a batch) and returns the messages it
// produced. Commands that wait, such as the cursor blink, are abandoned.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if batch, ok := msg.(tea.BatchMsg); ok {
			var out []tea.Msg
			for _, c := range batch {
				out = append(out, runCmd(c)...)
			}
			return out
		}
		return []tea.Msg{msg}
	case <-time.After(30 * time.Millisecond):
		return nil
	}
}

// filterBy opens the "/" prompt and types term, feeding the list's filter
// results back to the model like the Bubble Tea runtime would.
func filterBy(m tea.Model, term string) tea.Model {
	m = press(m, "/")
	for _, r := range term {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		for _, msg := range runCmd(cmd) {
			if _, ok := msg.(list.FilterMatchesMsg); ok {
				m, _ = m.Update(msg)
			}
		}
	}
	return m
}

// addWith adds a secret through the form with notes and tags.
func addWith(m tea.Model, name, notes, tags string) tea.Model {
	return press(m, "n", name, "enter", "X", "enter", "v", "tab", "enter", notes, "enter", tags, "enter")
}

func TestFilterFindsAWordThatOnlyExistsInNotes(t *testing.T) {
	m := addWith(newTestModel(t), "alpha", "rotate quarterly", "")
	m = addWith(m, "bravo", "see the wiki", "")
	if got := visibleTitles(filterBy(m, "quarterly")); !sameStrings(got, []string{"alpha"}) {
		t.Errorf("filter on a notes-only word shows %v, want [alpha]", got)
	}
}

func TestFilterFindsATag(t *testing.T) {
	m := addWith(newTestModel(t), "alpha", "", "payments")
	m = addWith(m, "bravo", "", "")
	if got := visibleTitles(filterBy(m, "payments")); !sameStrings(got, []string{"alpha"}) {
		t.Errorf("filter on a tag shows %v, want [alpha]", got)
	}
}

func TestFilterNeverSearchesTheSecretValue(t *testing.T) {
	m := add(newTestModel(t), "alpha", "X", "UNIQUE-VALUE-TEXT")
	for _, it := range m.(model).list.Items() {
		if v := it.FilterValue(); strings.Contains(v, "UNIQUE") || strings.Contains(v, "VALUE") {
			t.Errorf("FilterValue %q contains the secret value", v)
		}
	}
	if got := visibleTitles(filterBy(m, "UNIQUE")); len(got) != 0 {
		t.Errorf("filtering by a word of the value shows %v", got)
	}
}

// The row highlights matches inside the title only; a match in notes or tags
// has indexes past the title and must not mark any title character.
func TestMatchInNotesDoesNotHighlightTheTitle(t *testing.T) {
	m := addWith(newTestModel(t), "alpha", "rotate quarterly", "")
	m = filterBy(m, "quarterly")
	mm := m.(model)
	if len(mm.list.VisibleItems()) != 1 {
		t.Fatalf("shows %v, want one match", visibleTitles(m))
	}
	for _, idx := range mm.list.MatchesForItem(0) {
		if idx < len([]rune("alpha")) {
			t.Errorf("match index %d falls inside the title %q for a notes-only match", idx, "alpha")
		}
	}
}
