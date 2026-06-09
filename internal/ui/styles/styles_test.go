package styles

import (
	"strings"
	"testing"

	"tskr/internal/store"
)

func TestPriorityBadgeText(t *testing.T) {
	cases := []struct {
		p    store.Priority
		want string
	}{
		{store.PriorityLow, "LOW"},
		{store.PriorityMedium, "MED"},
		{store.PriorityHigh, "HIGH"},
		{store.PriorityUrgent, "URG"},
		{store.PriorityNone, "———"},
	}
	for _, c := range cases {
		if got := PriorityBadge(c.p); !strings.Contains(got, c.want) {
			t.Errorf("PriorityBadge(%q) = %q, want it to contain %q", c.p, got, c.want)
		}
	}
}

func TestStatusLabelText(t *testing.T) {
	if !strings.Contains(StatusLabel(store.StatusInProgress), "In Progress") {
		t.Error("in_progress label")
	}
	if !strings.Contains(StatusLabel(store.StatusPending), "Pending") {
		t.Error("pending label")
	}
	if !strings.Contains(StatusLabel(store.StatusDone), "Done") {
		t.Error("done label")
	}
}
