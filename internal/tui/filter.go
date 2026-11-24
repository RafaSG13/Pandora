package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

// Indexes of item.FilterValue's fields.
const (
	filterTitle = iota
	filterUsername
	filterCategory
	filterTags
	filterNotes
	filterFields
)

// qualifiedFilter is the list's FilterFunc. Tokens "tag:x", "cat:x" (or "category:x") and "user:x" keep only
// items whose tag / category / username contains x (case-insensitive; several qualifiers must all hold;
// an empty value like "tag:" is ignored so typing never blanks the list). Every other token is free text,
// ranked with list.DefaultFilter over the item's fields joined by spaces, exactly as before.
// A qualifier value is one word: quoting is not supported.
func qualifiedFilter(term string, targets []string) []list.Rank {
	var free []string
	qualifiers := map[string][]string{} // field name -> lowercase values
	for _, tok := range strings.Fields(term) {
		name, value, ok := strings.Cut(tok, ":")
		name = strings.ToLower(name)
		if !ok || (name != "tag" && name != "cat" && name != "category" && name != "user") {
			free = append(free, tok)
			continue
		}
		if name == "category" {
			name = "cat"
		}
		if value != "" {
			qualifiers[name] = append(qualifiers[name], strings.ToLower(value))
		}
	}

	var kept []int
	var joined []string
	for i, target := range targets {
		fields := strings.Split(target, fieldSep)
		if len(fields) != filterFields || !qualifiersHold(qualifiers, fields) {
			continue
		}
		kept = append(kept, i)
		joined = append(joined, strings.Join(fields, " "))
	}

	text := strings.Join(free, " ")
	if text == "" {
		ranks := make([]list.Rank, len(kept))
		for n, i := range kept {
			ranks[n] = list.Rank{Index: i}
		}
		return ranks
	}
	ranks := list.DefaultFilter(text, joined)
	for n := range ranks {
		ranks[n].Index = kept[ranks[n].Index]
	}
	return ranks
}

// qualifiersHold reports whether fields satisfy every qualifier.
func qualifiersHold(qualifiers map[string][]string, fields []string) bool {
	for name, values := range qualifiers {
		var field string
		switch name {
		case "tag":
			field = fields[filterTags]
		case "cat":
			field = fields[filterCategory]
		case "user":
			field = fields[filterUsername]
		}
		field = strings.ToLower(field)
		for _, v := range values {
			if !strings.Contains(field, v) {
				return false
			}
		}
	}
	return true
}
