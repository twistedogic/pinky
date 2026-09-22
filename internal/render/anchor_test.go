package render

import "testing"

func TestByteToLineInBlock(t *testing.T) {
	b := Block{Source: "alpha\nbravo\ncharlie"}
	cases := []struct {
		offset int
		want   int
	}{
		{0, 0},
		{6, 1},
		{99, 2}, // clamped to last line
	}
	for _, c := range cases {
		if got := byteToLineInBlock(b, c.offset); got != c.want {
			t.Errorf("byteToLineInBlock(%d) = %d want %d", c.offset, got, c.want)
		}
	}
}