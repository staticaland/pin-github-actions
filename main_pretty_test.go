package main

import "testing"

func TestIsFullSHA(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"lowercase hex 40", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"uppercase hex 40", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", true},
		{"mixed-case hex 40", "AaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAa", true},
		{"too short 39", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
		{"too long 41", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
		{"non-hex char g", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaagaaaaaa", false},
	}
	for _, tc := range cases {
		if got := isFullSHA(tc.in); got != tc.ok {
			t.Fatalf("%s: isFullSHA(%q) = %v, want %v", tc.name, tc.in, got, tc.ok)
		}
	}
}

func TestPrettyRef(t *testing.T) {
	fortyHex := "1234567890abcdef1234567890abcdef12345678"
	if len(fortyHex) != 40 {
		t.Fatalf("test bug: fortyHex length = %d", len(fortyHex))
	}

	cases := []struct {
		name string
		in   string
		out  string
	}{
		{"empty", "", "(none)"},
		{"spaces", "   ", "(none)"},
		{"full sha shortened", fortyHex, fortyHex[:12] + "…"},
		{"non-hex 40 stays same", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"short ref unchanged", "v1.2.3", "v1.2.3"},
	}

	for _, tc := range cases {
		if got := prettyRef(tc.in); got != tc.out {
			t.Fatalf("%s: prettyRef(%q) = %q, want %q", tc.name, tc.in, got, tc.out)
		}
	}
}
