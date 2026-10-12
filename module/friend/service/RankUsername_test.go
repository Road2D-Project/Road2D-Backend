package service

import "testing"

func TestRankUsernameOrdersExactPrefixContains(t *testing.T) {
	cases := []struct {
		query string
		name  string
		score int
		ok    bool
	}{
		{query: "ann", name: "Ann", score: usernameExact, ok: true},
		{query: "ann", name: "Annette", score: usernamePrefix, ok: true},
		{query: "ann", name: "joanna", score: usernameContains, ok: true},
		{query: "ann", name: "bea", ok: false},
		{query: "", name: "ann", ok: false},
	}
	for _, tc := range cases {
		score, ok := rankUsername(tc.query, tc.name)
		if ok != tc.ok || (ok && score != tc.score) {
			t.Fatalf("rank(%q, %q) = %d %v", tc.query, tc.name, score, ok)
		}
	}
}

func TestLikeContainsEscapesWildcards(t *testing.T) {
	got := likeContains("100%")
	if got != `%100\%%` {
		t.Fatalf("got %q", got)
	}
}

func TestExactPhoneQueryKeepsOnlyAFullDigitString(t *testing.T) {
	if got := exactPhoneQuery(" 090-123-4567 "); got != "0901234567" {
		t.Fatalf("separators = %q", got)
	}
	if got := exactPhoneQuery("+84901234567"); got != "84901234567" {
		t.Fatalf("plus = %q", got)
	}
	if got := exactPhoneQuery("ann090"); got != "" {
		t.Fatalf("mixed = %q", got)
	}
	if got := exactPhoneQuery("0901"); got != "0901" {
		t.Fatalf("short digits stay exact, got %q", got)
	}
}
