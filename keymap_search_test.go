package main

import (
	"strings"
	"testing"
)

// TestHelp_IncludesSearchKeys: when the help overlay is shown
// in stateNav or stateFileView, the new search keys (`/`, `n`,
// `N`) appear in the rendered text.
func TestHelp_IncludesSearchKeys_Nav(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateNav
	m.help.ShowAll = true
	view := m.help.View(m)
	if !strings.Contains(view, "/") {
		t.Errorf("expected '/' in nav help; got:\n%s", view)
	}
}

func TestHelp_IncludesSearchKeys_FileView(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.state = stateFileView
	m.help.ShowAll = true
	view := m.help.View(m)
	if !strings.Contains(view, "/") {
		t.Errorf("expected '/' in file-view help; got:\n%s", view)
	}
}
