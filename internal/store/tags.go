package store

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxTags is the most tags one secret can carry.
	MaxTags = 10
	// MaxTagLen is the longest tag, in characters.
	MaxTagLen = 24
)

// ErrInvalidTag is returned for a tag that is too long, contains whitespace
// or a comma, or for a secret with too many tags.
var ErrInvalidTag = errors.New("invalid tag")

// NormalizeTags returns tags in their stored form: trimmed, lowercase,
// without empty entries or duplicates, sorted, so one tag never has two
// spellings. Control characters give ErrInvalidText; whitespace or a comma
// inside a tag, a tag longer than MaxTagLen, or more than MaxTags distinct
// tags give ErrInvalidTag. No tags yield nil, so the field is omitted when
// saved.
func NormalizeTags(tags []string) ([]string, error) {
	var out []string
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if err := ValidateText(t); err != nil {
			return nil, err
		}
		if strings.ContainsFunc(t, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
			return nil, fmt.Errorf("%w %q: no spaces or commas inside a tag", ErrInvalidTag, t)
		}
		if utf8.RuneCountInString(t) > MaxTagLen {
			return nil, fmt.Errorf("%w %q: longer than %d characters", ErrInvalidTag, t, MaxTagLen)
		}
		out = append(out, t)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) > MaxTags {
		return nil, fmt.Errorf("%w: at most %d tags per secret", ErrInvalidTag, MaxTags)
	}
	return out, nil
}

// ParseTags splits a comma-separated list, as typed in the form or given to
// --tag, and normalises it with NormalizeTags.
func ParseTags(s string) ([]string, error) {
	return NormalizeTags(strings.Split(s, ","))
}

// prepareMeta validates the text fields and replaces Tags by their normalised
// form, so every writer stores the same representation.
func prepareMeta(m *SecretMeta) error {
	for _, s := range []string{m.Title, m.Category, m.Notes} {
		if err := ValidateText(s); err != nil {
			return err
		}
	}
	tags, err := NormalizeTags(m.Tags)
	if err != nil {
		return err
	}
	m.Tags = tags
	return nil
}
