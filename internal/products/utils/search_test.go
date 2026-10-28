package utils

import (
	"strings"
	"testing"
)

func TestTSQuery(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \t\n", ""},
		{"single word", "oil", "oil:*"},
		{"multiple words", "cooking oil", "cooking:* & oil:*"},
		{"mixed separators", "  Sunseed   Cooking-Oil 2L ", "sunseed:* & cooking:* & oil:* & 2l:*"},
		{"drops one-character tokens", "a !!! b", ""},
		{"keeps digits", "2l", "2l:*"},
		{"strips tsquery operators", `dress & ! (oil) | "2l"`, "dress:* & oil:* & 2l:*"},
		{"unicode letters", "café crème", "café:* & crème:*"},
		{"caps term count", strings.Repeat("word ", 15), strings.Repeat("word:* & ", 9) + "word:*"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TSQuery(tc.in); got != tc.want {
				t.Fatalf("TSQuery(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
