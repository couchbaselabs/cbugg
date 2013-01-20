package main

import (
	"path/filepath"
	"testing"
)

// Verify we got all the templates at least loaded.
//
// If there's a problem parsing the templates, we won't actually even
// get this far.
func TestTemplateInit(t *testing.T) {
	files, err := filepath.Glob("templates/*")
	if err != nil {
		t.Fatalf("Error listing template: %v", err)
	}

	for _, f := range files {
		basename := filepath.Base(f)
		tmpl := templates.Lookup(basename)
		if tmpl == nil {
			t.Errorf("Error looking up template: %v", basename)
		}
	}

}
