package letterboxd

import "testing"

func TestParseRating(t *testing.T) {
	tests := []struct {
		in    string
		want  Rating
		stars string
		label string
	}{
		{"5", 10, "5", "★★★★★"},
		{"4.5", 9, "4.5", "★★★★½"},
		{" 3 ", 6, "3", "★★★"},
		{"0.5", 1, "0.5", "½"},
	}
	for _, tt := range tests {
		got, err := ParseRating(tt.in)
		if err != nil {
			t.Fatalf("ParseRating(%q): %v", tt.in, err)
		}
		if got != tt.want || got.Stars() != tt.stars || got.String() != tt.label {
			t.Errorf("ParseRating(%q) = %d (%s, %s), want %d (%s, %s)",
				tt.in, got, got.Stars(), got, tt.want, tt.stars, tt.label)
		}
	}
}

func TestParseRatingInvalid(t *testing.T) {
	for _, in := range []string{"", "0", "5.5", "4.2", "six"} {
		if _, err := ParseRating(in); err == nil {
			t.Errorf("ParseRating(%q) succeeded, want error", in)
		}
	}
}
