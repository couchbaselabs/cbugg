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
