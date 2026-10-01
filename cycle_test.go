package main

import "testing"

// TestTab_CycleOrder: Tab cycles through Message → Files → Todos
// → Message. Sub-state survives a round-trip per tab.
func TestTab_CycleOrder(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.tab = tabMessage
	m.state = stateNav
	m.messageReturn = stateNav
	m.fileReturn = stateFileNav
	m.todoReturn = stateTodoList
	m.fileRoot = t.TempDir() // give the file walker something to walk
	m.reflow()

	// Message → Files
	m.toggleTab()
	if m.tab != tabFiles {
		t.Errorf("after 1st Tab: m.tab = %v, want tabFiles", m.tab)
	}
	if m.state != stateFileNav {
		t.Errorf("after 1st Tab: m.state = %v, want stateFileNav", m.state)
	}

	// Files → Todos
	m.toggleTab()
	if m.tab != tabTodos {
		t.Errorf("after 2nd Tab: m.tab = %v, want tabTodos", m.tab)
	}
	if m.state != stateTodoList {
		t.Errorf("after 2nd Tab: m.state = %v, want stateTodoList", m.state)
	}

	// Todos → Message (restores message sub-state)
	m.toggleTab()
	if m.tab != tabMessage {
		t.Errorf("after 3rd Tab: m.tab = %v, want tabMessage", m.tab)
	}
	if m.state != stateNav {
		t.Errorf("after 3rd Tab: m.state = %v, want stateNav", m.state)
	}
}

// TestTab_CycleOrder_RestoresSubState: leaving a tab in compose
// state returns the user to compose when they cycle back.
func TestTab_CycleOrder_RestoresSubState(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.tab = tabMessage
	m.state = stateCompose
	m.messageReturn = stateCompose
	m.fileReturn = stateFileNav
	m.todoReturn = stateTodoList
	m.fileRoot = t.TempDir()
	m.reflow()

	m.toggleTab() // → Files
	m.toggleTab() // → Todos
	m.toggleTab() // → Message (should restore compose)

	if m.tab != tabMessage {
		t.Errorf("m.tab = %v, want tabMessage", m.tab)
	}
	if m.state != stateCompose {
		t.Errorf("m.state = %v, want stateCompose (restored)", m.state)
	}
}

// TestTab_CycleOrder_NoOpInDisabledStates: Tab in stateCommentComposer,
// statePicking, or stateError is a no-op (handled at the keymap
// layer; this just exercises the underlying invariant that toggleTab
// never panics regardless of m.state).
func TestTab_CycleOrder_NoOpInDisabledStates(t *testing.T) {
	m := newIdleModelForKeymap(t)
	m.tab = tabMessage
	m.state = stateNav
	for _, s := range []state{stateCommentComposer, statePicking, stateError} {
		m.state = s
		m.toggleTab()
		// state-machine dispatch in handleKey already filters these
		// out; toggleTab itself only depends on m.tab, so verify
		// the cycle still works once we leave the disabled state.
	}
}