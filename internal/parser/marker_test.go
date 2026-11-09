package parser

import "testing"

func TestSplitMarker(t *testing.T) {
	cases := []struct {
		in, before, after string
		count             int
	}{
		{"A is a way to build :: some thing.", "A is a way to build", "some thing.", 1},
		{"No marker here.", "No marker here.", "", 0},
		{"Uses `std::vector` only.", "Uses `std::vector` only.", "", 0},
		{"Uses ``a::b `c` `` :: x", "Uses ``a::b `c` ``", "x", 1},
		{"Unclosed `a :: b", "Unclosed `a", "b", 1},
		{"a :: b :: c", "a", "b :: c", 2},
	}
	for _, c := range cases {
		b, a, n := SplitMarker(c.in)
		if b != c.before || a != c.after || n != c.count {
			t.Errorf("SplitMarker(%q) = (%q, %q, %d), want (%q, %q, %d)",
				c.in, b, a, n, c.before, c.after, c.count)
		}
	}
}

func TestBoldSpans(t *testing.T) {
	spans := BoldSpans("a **b** `**c**` **d e**")
	if len(spans) != 2 || spans[0].Text != "b" || spans[1].Text != "d e" {
		t.Fatalf("spans = %+v", spans)
	}
	if got := StripBold("a **b** `**c**` **d e**"); got != "a b `**c**` d e" {
		t.Errorf("StripBold = %q", got)
	}
	if got := ReplaceBold("a **b** and **c**", 1, "___"); got != "a b and ___" {
		t.Errorf("ReplaceBold = %q", got)
	}
}
