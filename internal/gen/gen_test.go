package gen

import (
	"errors"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	for _, cs := range Charsets {
		for _, n := range append([]int{MinLength, MaxLength}, Lengths...) {
			a, err := Generate(cs, n)
			if err != nil || len(a) != n {
				t.Fatalf("%s/%d: %q err=%v", cs.Key, n, a, err)
			}
			if !hasEveryClass(a, cs.Classes) {
				t.Errorf("%s/%d: %q misses a character class", cs.Key, n, a)
			}
			if strings.Trim(a, strings.Join(cs.Classes, "")) != "" {
				t.Errorf("%s/%d: %q has characters outside the charset", cs.Key, n, a)
			}
			if b, _ := Generate(cs, n); a == b {
				t.Errorf("%s/%d generated the same secret twice", cs.Key, n)
			}
		}
	}
}

func TestGenerateRejectsOutOfRangeLengths(t *testing.T) {
	for _, n := range []int{-1, 0, MinLength - 1, MaxLength + 1} {
		if _, err := Generate(Charsets[0], n); !errors.Is(err, ErrLength) {
			t.Errorf("Generate(%d) err = %v, want ErrLength", n, err)
		}
	}
}

func TestLookup(t *testing.T) {
	if cs, ok := Lookup("hex"); !ok || cs.Key != "hex" {
		t.Errorf("Lookup(hex) = %v, %v", cs, ok)
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup of an unknown key succeeded")
	}
}
