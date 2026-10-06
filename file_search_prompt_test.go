package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFileSearch_PromptRendersWhileActive(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	m = keyModelVal(t, m, "/")
	for _, r := range "alpha" {
		m = keyModelVal(t, m, string(r))
	}
	view := m.fileViewView()
	if !strings.Contains(view, "/alpha") {
		t.Errorf("expected view to contain '/alpha'; got:\n%s", view)
	}
}

func TestFileSearch_PromptHiddenWhenInactive(t *testing.T) {
	root, rel := writeUTF8Fixture(t, "main.go", "alpha\nbeta\n")
	pm := openFileInFixture(t, root, rel)
	m := *pm
	view := m.fileViewView()
	if strings.Contains(view, "/alpha") {
		t.Errorf("expected no '/alpha' in view; got:\n%s", view)
	}
	_ = tea.KeyEsc
}
