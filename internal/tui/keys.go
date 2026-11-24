package tui

import "strings"

// keyMap resolves browse-mode key strings (tea.KeyMsg.String()) to the action
// names of config.KeyActions.
type keyMap struct {
	byKey map[string]string
	bound map[string][]string
}

func newKeyMap(bound map[string][]string) keyMap {
	byKey := make(map[string]string, len(bound)*2)
	for action, keys := range bound {
		for _, k := range keys {
			byKey[k] = action
		}
	}
	return keyMap{byKey: byKey, bound: bound}
}

// action returns the action bound to key, or "" when it is unbound.
func (k keyMap) action(key string) string { return k.byKey[key] }

// label lists the keys of action as shown in hints, e.g. "enter/c".
func (k keyMap) label(action string) string { return strings.Join(k.bound[action], "/") }

// first returns the primary key of action.
func (k keyMap) first(action string) string {
	if keys := k.bound[action]; len(keys) > 0 {
		return keys[0]
	}
	return ""
}
