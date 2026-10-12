package service

import (
	"strings"
	"unicode"
)

const (
	usernameExact    = 0
	usernamePrefix   = 1
	usernameContains = 2

	// phoneExact sorts ahead of every username score.
	// It is only assigned when the stored number equals the full query.
	phoneExact = -1

	// searchLimit is the page returned to the client.
	// Upgrade later: cursor pagination instead of this hard cut.
	searchLimit = 20

	maxSearchQueryLen = 64
)

// rankUsername scores a case-insensitive username against a normalized query.
//
// This is a substring ranker, not typo-tolerant fuzzy search.
// Upgrade later:
//   - Postgres pg_trgm similarity() plus a GIN index, so "ngyen" can match "nguyen"
//   - rank by that similarity score instead of exact / prefix / contains
//   - search a display name when one exists; do not add email to public search
//   - cursor pagination instead of searchLimit
//   - rate-limit the endpoint, and exclude blocked users when a block list exists
func rankUsername(query, username string) (int, bool) {
	name := strings.ToLower(strings.TrimSpace(username))
	switch {
	case query == "":
		return 0, false
	case name == query:
		return usernameExact, true
	case strings.HasPrefix(name, query):
		return usernamePrefix, true
	case strings.Contains(name, query):
		return usernameContains, true
	default:
		return 0, false
	}
}

func normalizeSearchQuery(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// likeContains builds an ILIKE pattern. % and _ in the query are literals.
func likeContains(query string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	return "%" + escaped + "%"
}

// exactPhoneQuery returns digits to compare with users.number_phone.
// The match is equality of the whole stored number, never a prefix or substring.
// Spaces, hyphens, and a leading plus are stripped only when every other
// character is a digit. A query that contains letters is not a phone lookup.
// Do not change the caller to LIKE or pg_trgm on number_phone.
func exactPhoneQuery(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var digits strings.Builder
	for _, r := range trimmed {
		switch {
		case unicode.IsDigit(r):
			digits.WriteRune(r)
		case r == ' ' || r == '-' || r == '+':
			continue
		default:
			return ""
		}
	}
	return digits.String()
}
