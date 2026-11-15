package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestCategoryColourIsStableAndCaseInsensitive(t *testing.T) {
	if categoryIndex("TOKENS") != categoryIndex("  tokens ") {
		t.Error("category colour should ignore case and surrounding spaces")
	}
	seen := map[int]bool{}
	for _, c := range []string{"TOKENS", "INFRA", "DB", "CI", "PERSONAL", "WORK", "API", "CLOUD"} {
		seen[categoryIndex(c)] = true
	}
	if len(seen) < 3 {
		t.Errorf("categories collapse into %d colours; palette is not spreading them", len(seen))
	}
}

// Every rendered screen must fit the terminal, or borders wrap and break.
func TestViewFitsTerminal(t *testing.T) {
	sizes := [][2]int{{60, 18}, {70, 24}, {100, 30}, {200, 50}}
	setups := map[string]func(tea.Model) tea.Model{
		"empty": func(m tea.Model) tea.Model { return m },
		"list": func(m tea.Model) tea.Model {
			return add(add(add(m, "a-rather-long-secret-name-that-must-be-truncated", "A-LONG-CATEGORY-NAME", "x"), "b", "", "y"), "c", "INFRA", "z")
		},
		"revealed": func(m tea.Model) tea.Model { return press(add(m, "a", "X", "secret"), "v") },
		"form":     func(m tea.Model) tea.Model { return press(m, "n") },
		"confirm":  func(m tea.Model) tea.Model { return press(add(m, "a", "X", "s"), "d") },
		"status":   func(m tea.Model) tea.Model { return add(m, "a", "X", "s") },
	}

	for name, setup := range setups {
		for _, sz := range sizes {
			m := newTestModel(t)
			m = setup(m)
			m, _ = m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})

			lines := strings.Split(m.View(), "\n")
			if len(lines) > sz[1] {
				t.Errorf("%s %dx%d: %d lines, want <= %d", name, sz[0], sz[1], len(lines), sz[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > sz[0] {
					t.Errorf("%s %dx%d: line %d is %d wide", name, sz[0], sz[1], i, w)
					break
				}
			}
		}
	}
}

func TestTooSmallTerminalShowsMessage(t *testing.T) {
	m := add(newTestModel(t), "a", "X", "s")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if v := m.View(); !strings.Contains(v, "Terminal too small") {
		t.Errorf("view = %q, want size warning", v)
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if v := m.View(); strings.Contains(v, "Terminal too small") {
		t.Error("warning should disappear once the terminal is large enough")
	}
}
