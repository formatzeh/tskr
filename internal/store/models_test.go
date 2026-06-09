package store

import "testing"

func TestNormalizeTags(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"A, b ,a", "a,b"},
		{"  Work , urgent,", "work,urgent"},
	}
	for _, c := range cases {
		if got := NormalizeTags(c.in); got != c.want {
			t.Errorf("NormalizeTags(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOverdue(t *testing.T) {
	today := "2026-06-09"
	cases := []struct {
		task Task
		want bool
	}{
		{Task{DueDate: "2026-06-08", Status: StatusPending}, true},
		{Task{DueDate: "2026-06-09", Status: StatusPending}, false},
		{Task{DueDate: "2026-06-08", Status: StatusDone}, false},
		{Task{DueDate: "", Status: StatusPending}, false},
	}
	for i, c := range cases {
		if got := c.task.Overdue(today); got != c.want {
			t.Errorf("case %d: got %v, want %v", i, got, c.want)
		}
	}
}
