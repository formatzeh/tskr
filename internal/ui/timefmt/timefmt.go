// Package timefmt parses and formats logged-time durations.
package timefmt

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var durRe = regexp.MustCompile(`^(?:(\d+)\s*h)?\s*(?:(\d+)\s*m)?$`)

// ParseDuration accepts "90m", "2h", "1h30m", "1h 30m" (case-insensitive)
// and returns total minutes (> 0).
func ParseDuration(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	m := durRe.FindStringSubmatch(s)
	if m == nil || (m[1] == "" && m[2] == "") {
		return 0, fmt.Errorf("use formats like 90m, 2h, 1h30m")
	}
	hours, _ := strconv.Atoi("0" + m[1])
	mins, _ := strconv.Atoi("0" + m[2])
	total := hours*60 + mins
	if total <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return total, nil
}

// FormatMinutes renders minutes as "2h 30m", "2h", or "45m".
func FormatMinutes(min int) string {
	if min <= 0 {
		return "0m"
	}
	h, m := min/60, min%60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
