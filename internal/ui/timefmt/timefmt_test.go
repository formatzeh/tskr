package timefmt

import "testing"

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"90m", 90, false},
		{"2h", 120, false},
		{"1h30m", 90, false},
		{"1h 30m", 90, false},
		{" 1H 5M ", 65, false},
		{"", 0, true},
		{"abc", 0, true},
		{"0m", 0, true},
		{"90", 0, true},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("ParseDuration(%q) = %d, %v; want %d, err=%v", c.in, got, err, c.want, c.wantErr)
		}
	}
}

func TestFormatMinutes(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{150, "2h 30m"},
		{120, "2h"},
		{45, "45m"},
		{0, "0m"},
	}
	for _, c := range cases {
		if got := FormatMinutes(c.in); got != c.want {
			t.Errorf("FormatMinutes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
