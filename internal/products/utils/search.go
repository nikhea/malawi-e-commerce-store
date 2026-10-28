package utils

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxSearchTerms caps the number of lexemes in one query. Ten words is
// already more specific than most shoppers type and keeps the generated
// tsquery small.
const maxSearchTerms = 10

// TSQuery turns free text into a safe prefix tsquery:
//
//	"cooking oil" -> "cooking:* & oil:*"
//
// Prefix matching makes search-as-you-type work ("cook" finds "cooking",
// "sun" finds "Sunseed"). Every character outside letters and digits is a
// separator, so user input can never inject tsquery operators. An empty
// result means there is nothing searchable left; callers fall back to the
// substring path.
func TSQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	terms := strings.FieldsFunc(strings.ToLower(raw), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	out := make([]string, 0, len(terms))
	for _, term := range terms {
		// Single characters are noise ("a", "s"); they would match too
		// much through prefix search.
		if utf8.RuneCountInString(term) < 2 {
			continue
		}
		out = append(out, term+":*")
		if len(out) == maxSearchTerms {
			break
		}
	}
	return strings.Join(out, " & ")
}
