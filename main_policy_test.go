package main

import "testing"

func TestParsePolicy(t *testing.T) {
	cases := []struct {
		in       string
		want     UpdatePolicy
		wantErr  bool
	}{
		{"", UpdatePolicyMajor, false},
		{"major", UpdatePolicyMajor, false},
		{"latest-major", UpdatePolicyMajor, false},
		{"latest", UpdatePolicyMajor, false},
		{"same-major", UpdatePolicySameMajor, false},
		{"stay-major", UpdatePolicySameMajor, false},
		{"minor", UpdatePolicySameMajor, false},
		{"patch", UpdatePolicySameMajor, false},
		{"requested", UpdatePolicyRequested, false},
		{"exact", UpdatePolicyRequested, false},
		{"pin-requested", UpdatePolicyRequested, false},
		{"unknown-policy", UpdatePolicyMajor, true},
	}

	for _, tc := range cases {
		got, err := parsePolicy(tc.in)
		if tc.wantErr && err == nil {
			t.Fatalf("parsePolicy(%q) expected error, got nil", tc.in)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("parsePolicy(%q) unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parsePolicy(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
