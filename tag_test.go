package main

import (
	"testing"
)

func TestNewTags(t *testing.T) {
	tests := []struct {
		Old, New string
		Exp      []string
	}{
		{"", "", []string{}},
		{"", "hi", []string{"hi"}},
		{"", "hi,there", []string{"hi", "there"}},
		{"hi", "hi,there", []string{"there"}},
		{"hi,there", "hi", []string{}},
	}

	for _, x := range tests {
		foundm := map[string]bool{}
		for _, s := range newTags(x.Old, x.New) {
			foundm[s] = true
		}
		if len(foundm) == len(x.Exp) {
			for _, e := range x.Exp {
				if !foundm[e] {
					t.Errorf("Expected %v on %v -> %v, got %v",
						x.Old, x.New, x.Exp, foundm)
					continue
				}
			}
		} else {
			t.Errorf("Expected %v on %v -> %v, got %v (%v/%v)",
				x.Old, x.New, x.Exp, foundm,
				len(foundm), len(x.Exp))
		}
	}
}
