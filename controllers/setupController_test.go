package controllers

import "testing"

func TestSetupKeyMatching(t *testing.T) {
	cases := []struct {
		provided, expected string
		want               bool
	}{
		{"s3cret-key", "s3cret-key", true},
		{"S3cret-key", "s3cret-key", false}, // case matters
		{"s3cret-key ", "s3cret-key", false},
		{"", "s3cret-key", false},
		{"anything", "", false}, // no key configured: nothing matches, even an empty one
		{"", "", false},
	}
	for _, tc := range cases {
		if got := setupKeyMatches(tc.provided, tc.expected); got != tc.want {
			t.Errorf("setupKeyMatches(%q, %q) = %v, want %v", tc.provided, tc.expected, got, tc.want)
		}
	}
}
