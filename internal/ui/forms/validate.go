package forms

import (
	"strings"
	"time"

	"tskr/internal/ui/timefmt"
)

func Required(s string) string {
	if strings.TrimSpace(s) == "" {
		return "required"
	}
	return ""
}

func OptionalDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "use YYYY-MM-DD"
	}
	return ""
}

func OptionalPriority(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "low", "medium", "high", "urgent":
		return ""
	}
	return "one of: low, medium, high, urgent — or empty"
}

func ValidDuration(s string) string {
	if _, err := timefmt.ParseDuration(s); err != nil {
		return err.Error()
	}
	return ""
}
