package main

import "testing"

func TestPrettyRef(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want string
	}{
		{"empty string", "", "(none)"},
		{"whitespace only", "   ", "(none)"},
		{"40-hex SHA", "1234567890abcdef1234567890abcdef12345678", "1234567890ab…"},
		{"40-hex SHA uppercase", "1234567890ABCDEF1234567890ABCDEF12345678", "1234567890AB…"},
		{"mixed case SHA", "1234567890AbCdEf1234567890AbCdEf12345678", "1234567890Ab…"},
		{"39 chars", "1234567890abcdef1234567890abcdef1234567", "1234567890abcdef1234567890abcdef1234567"},
		{"41 chars", "1234567890abcdef1234567890abcdef123456789", "1234567890abcdef1234567890abcdef123456789"},
		{"non-hex chars", "1234567890abcdef1234567890abcdef1234567g", "1234567890abcdef1234567890abcdef1234567g"},
		{"tag v4.2.0", "v4.2.0", "v4.2.0"},
		{"branch main", "main", "main"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := prettyRef(tc.ref)
			if got != tc.want {
				t.Errorf("prettyRef(%q) = %q, want %q", tc.ref, got, tc.want)
			}
		})
	}
}

func TestIsFullSHA(t *testing.T) {
	cases := []struct {
		name string
		sha  string
		want bool
	}{
		{"valid 40-hex lowercase", "1234567890abcdef1234567890abcdef12345678", true},
		{"valid 40-hex uppercase", "1234567890ABCDEF1234567890ABCDEF12345678", true},
		{"valid 40-hex mixed case", "1234567890AbCdEf1234567890AbCdEf12345678", true},
		{"39 chars", "1234567890abcdef1234567890abcdef1234567", false},
		{"41 chars", "1234567890abcdef1234567890abcdef123456789", false},
		{"empty string", "", false},
		{"non-hex char g", "1234567890abcdef1234567890abcdef1234567g", false},
		{"non-hex char z", "1234567890abcdef1234567890abcdef1234567z", false},
		{"space in middle", "1234567890abcdef 234567890abcdef12345678", false},
		{"all zeros", "0000000000000000000000000000000000000000", true},
		{"all f's", "ffffffffffffffffffffffffffffffffffffffff", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isFullSHA(tc.sha)
			if got != tc.want {
				t.Errorf("isFullSHA(%q) = %v, want %v", tc.sha, got, tc.want)
			}
		})
	}
}

func TestIsMovingMajorTag(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want bool
	}{
		{"v4", "v4", true},
		{"4", "4", true},
		{"v10", "v10", true},
		{"123", "123", true},
		{"v4.2", "v4.2", false},
		{"v4.2.0", "v4.2.0", false},
		{"4.2", "4.2", false},
		{"main", "main", false},
		{"1234567890abcdef1234567890abcdef12345678", "1234567890abcdef1234567890abcdef12345678", false},
		{"v4-alpha", "v4-alpha", false},
		{"v", "v", false},
		{"", "", false},
		{"v0", "v0", true},
		{"0", "0", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isMovingMajorTag(tc.ref)
			if got != tc.want {
				t.Errorf("isMovingMajorTag(%q) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestParseMajor(t *testing.T) {
	cases := []struct {
		name      string
		ref       string
		wantMajor int
		wantOk    bool
	}{
		{"v4", "v4", 4, true},
		{"4", "4", 4, true},
		{"v10", "v10", 10, true},
		{"123", "123", 123, true},
		{"v4.2.2", "v4.2.2", 4, true},
		{"4.2.2", "4.2.2", 4, true},
		{"v1.0.0-alpha", "v1.0.0-alpha", 1, true},
		{"main", "main", 0, false},
		{"1234567890abcdef1234567890abcdef12345678", "1234567890abcdef1234567890abcdef12345678", 0, false},
		{"v4-alpha", "v4-alpha", 4, true},
		{"", "", 0, false},
		{"v", "v", 0, false},
		{"invalid.version", "invalid.version", 0, false},
		{"v0", "v0", 0, true},
		{"0", "0", 0, true},
		{"v0.1.0", "v0.1.0", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMajor, gotOk := parseMajor(tc.ref)
			if gotMajor != tc.wantMajor || gotOk != tc.wantOk {
				t.Errorf("parseMajor(%q) = (%d, %v), want (%d, %v)", tc.ref, gotMajor, gotOk, tc.wantMajor, tc.wantOk)
			}
		})
	}
}

func TestNormalizeMajorRef(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want string
	}{
		{"4", "4", "v4"},
		{"v4", "v4", "v4"},
		{"10", "10", "v10"},
		{"v10", "v10", "v10"},
		{"0", "0", "v0"},
		{"v0", "v0", "v0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeMajorRef(tc.ref)
			if got != tc.want {
				t.Errorf("normalizeMajorRef(%q) = %q, want %q", tc.ref, got, tc.want)
			}
		})
	}
}

func TestComputeLineCol(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		offset   int
		wantLine int
		wantCol  int
	}{
		{"start of file", "hello world", 0, 1, 1},
		{"mid first line", "hello world", 5, 1, 6},
		{"end of first line", "hello world", 11, 1, 12},
		{"start of second line", "hello\nworld", 6, 2, 1},
		{"mid second line", "hello\nworld", 8, 2, 3},
		{"multiple lines", "line1\nline2\nline3", 12, 3, 1},
		{"empty string", "", 0, 1, 1},
		{"empty lines", "\n\n\n", 2, 3, 1},
		{"negative offset", "hello", -1, 1, 1},
		{"offset beyond content", "hello", 10, 1, 6},
		{"windows line endings", "line1\r\nline2", 7, 2, 1},
		{"mixed line endings", "line1\nline2\r\nline3", 13, 3, 1},
		{"very long line", "hello world this is a very long line with many characters", 25, 1, 26},
		{"multi-line with varying lengths", "short\na much longer line here\nend", 6, 2, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotLine, gotCol := computeLineCol(tc.content, tc.offset)
			if gotLine != tc.wantLine || gotCol != tc.wantCol {
				t.Errorf("computeLineCol(%q, %d) = (%d, %d), want (%d, %d)", tc.content, tc.offset, gotLine, gotCol, tc.wantLine, tc.wantCol)
			}
		})
	}
}