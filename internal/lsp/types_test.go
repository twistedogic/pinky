package lsp

import "testing"

// TestKind_DiscriminatesThree: only three kinds, in this order.
// Spec D4 / D5 / D8 — the bridge switches on Kind to choose the
// reply shape and the model layer uses it to decide which Msg
// type to construct.
func TestKind_DiscriminatesThree(t *testing.T) {
	if KindDefinition != 0 || KindReferences != 1 || KindHover != 2 {
		t.Errorf("expected Kind ordering Definition=0, References=1, Hover=2; got %d,%d,%d",
			KindDefinition, KindReferences, KindHover)
	}
}

// TestResult_DefaultFields: zero-value Result must not satisfy
// "has content" checks (Locations nil, Hover nil, no error).
// Used by the model layer to decide between silent and visible.
func TestResult_DefaultFields(t *testing.T) {
	var r Result
	if r.Locations != nil {
		t.Errorf("expected nil Locations on zero value")
	}
	if r.Hover != nil {
		t.Errorf("expected nil Hover on zero value")
	}
	if r.Err != nil {
		t.Errorf("expected nil Err on zero value")
	}
	if r.ErrServerMissing {
		t.Errorf("expected ErrServerMissing=false on zero value")
	}
}