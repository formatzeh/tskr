// Package fuzzy implements case-insensitive subsequence matching.
package fuzzy

import "strings"

// Score returns a match score (lower is better) or -1 when query is not
// a subsequence of target. The score is the sum of gaps between matched
// characters plus the offset of the first match, so earlier and denser
// matches rank first. An empty query matches everything with score 0.
func Score(query, target string) int {
	q := strings.ToLower(query)
	t := strings.ToLower(target)
	if q == "" {
		return 0
	}
	score, last, ti := 0, -1, 0
	for qi := 0; qi < len(q); qi++ {
		idx := strings.IndexByte(t[ti:], q[qi])
		if idx < 0 {
			return -1
		}
		pos := ti + idx
		if last >= 0 {
			score += pos - last - 1
		} else {
			score += pos
		}
		last = pos
		ti = pos + 1
	}
	return score
}

func Match(query, target string) bool { return Score(query, target) >= 0 }
