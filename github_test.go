package main

import (
	"testing"
)

func TestCleanupPatchTitle(t *testing.T) {
	tests := []struct {
		In, Out string
	}{
		{"", ""},
		{"testing", "testing"},
		{"I am a sentence.", "I-am-a-sentence"},
		{`I have "quotes"`, "I-have-quotes"},
	}

	for _, x := range tests {
		got := cleanupPatchTitle(x.In)
		if got != x.Out {
			t.Errorf("On %v, expected %v, got %v",
				x.In, x.Out, got)
		}
	}
}

func TestReferencesFromGithub(t *testing.T) {
	empty := []githubCBRef{}

	tests := []struct {
		Msg string
		Res []githubCBRef
	}{
		{"", empty},
		{"Cbugg: bug-134", []githubCBRef{{"bug-134", false}}},
		{"Cbugg: close bug-134", []githubCBRef{{"bug-134", true}}},
		{"Cbugg: closed bug-134", []githubCBRef{{"bug-134", true}}},
		{"Did some stuff\n\nCbugg: bug-134", []githubCBRef{{"bug-134", false}}},
		{"Did some stuff\n\n  Cbugg: bug-134", []githubCBRef{{"bug-134", false}}},
		{"Did some stuff\n\n  cbugg: bug-134", []githubCBRef{{"bug-134", false}}},
		{"Cbugg: close bug-134 bug-135", []githubCBRef{{"bug-134", true},
			{"bug-135", true}}},
		{"Cbugg: close bug-134\ncbugg: bug-135",
			[]githubCBRef{{"bug-134", true}, {"bug-135", false}}},
	}

	for _, x := range tests {
		got := extractRefsFromGithub(x.Msg)
		if len(got) != len(x.Res) {
			t.Errorf("On %v, expected %v, got %v",
				x.Msg, x.Res, got)
			continue
		}

		for i := range x.Res {
			if got[i].bugid != x.Res[i].bugid {
				t.Errorf("Expected bugid %v, got %v on %v",
					x.Res[i].bugid, got[i].bugid, x.Msg)
			}
			if got[i].closed != x.Res[i].closed {
				t.Errorf("Expected closed=%v, got %v on %v",
					x.Res[i].closed, got[i].closed, x.Msg)
			}
		}
	}
}
