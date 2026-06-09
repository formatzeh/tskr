package fuzzy

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		q, target string
		want      bool
	}{
		{"", "anything", true},
		{"fb", "fix bug", true},
		{"FB", "fix bug", true},
		{"bgf", "fix bug", false},
		{"xyz", "fix bug", false},
		{"bug", "fix bug", true},
	}
	for _, c := range cases {
		if got := Match(c.q, c.target); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.q, c.target, got, c.want)
		}
	}
}

func TestScoreRanksTighterMatchesLower(t *testing.T) {
	if Score("fix", "fix bug") >= Score("fix", "freight index") {
		t.Error("contiguous match should score lower (better) than spread match")
	}
	if Score("zz", "fix bug") != -1 {
		t.Error("non-match must return -1")
	}
}
