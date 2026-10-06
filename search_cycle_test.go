package main

import (
	"testing"

	"github.com/twistedogic/pinky/internal/render"
)

func TestCycleSearchHit_WrapsForward(t *testing.T) {
	hits := []render.Hit{
		{LineIdx: 0, ByteA: 0, ByteC: 1},
		{LineIdx: 1, ByteA: 0, ByteC: 1},
		{LineIdx: 2, ByteA: 0, ByteC: 1},
	}
	cur := 2
	if got := cycleSearchHit(+1, hits, &cur); got != 0 {
		t.Errorf("wrap forward from 2: got %d, want 0", got)
	}
}

func TestCycleSearchHit_WrapsBack(t *testing.T) {
	hits := []render.Hit{
		{LineIdx: 0, ByteA: 0, ByteC: 1},
		{LineIdx: 1, ByteA: 0, ByteC: 1},
		{LineIdx: 2, ByteA: 0, ByteC: 1},
	}
	cur := 0
	if got := cycleSearchHit(-1, hits, &cur); got != 2 {
		t.Errorf("wrap back from 0: got %d, want 2", got)
	}
}

func TestCycleSearchHit_EmptyHitsNoOp(t *testing.T) {
	var hits []render.Hit
	cur := 5
	if got := cycleSearchHit(+1, hits, &cur); got != -1 {
		t.Errorf("empty hits: got %d, want -1", got)
	}
	if cur != -1 {
		t.Errorf("cur should be reset to -1 on empty hits; got %d", cur)
	}
}

func TestCycleSearchHit_Advances(t *testing.T) {
	hits := []render.Hit{
		{LineIdx: 0, ByteA: 0, ByteC: 1},
		{LineIdx: 1, ByteA: 0, ByteC: 1},
		{LineIdx: 2, ByteA: 0, ByteC: 1},
	}
	cur := 0
	if got := cycleSearchHit(+1, hits, &cur); got != 1 {
		t.Errorf("advance from 0: got %d, want 1", got)
	}
	cur = 1
	if got := cycleSearchHit(-1, hits, &cur); got != 0 {
		t.Errorf("retreat from 1: got %d, want 0", got)
	}
}
