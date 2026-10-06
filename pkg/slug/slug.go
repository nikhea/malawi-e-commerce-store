// Package slug generates URL-safe ASCII slugs ("Cooking Oil" →
// "cooking-oil"). Domain-free: shared by every module that slugs names.
package slug

import (
	"strings"
	"unicode"
)

// Make lowercases, swaps spaces/underscores for hyphens, drops anything
// outside [a-z0-9-], and collapses repeats. Non-ASCII letters (e.g.
// Chichewa diacritics) are dropped so slugs stay URL-safe.
func Make(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevHyphen := true // trim leading hyphens
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '_' || r == '-':
			if !prevHyphen {
				b.WriteRune('-')
				prevHyphen = true
			}
		case unicode.IsLetter(r):
			// Drop: keep slugs ASCII.
		default:
			// Drop punctuation, symbols, emoji.
		}
	}
	return strings.Trim(b.String(), "-")
}
