package main

import "testing"

func TestFilmSlug(t *testing.T) {
	tests := map[string]string{
		"parasite-2019": "parasite-2019",
		" https://letterboxd.com/film/parasite-2019/ ": "parasite-2019",
		"letterboxd.com/film/whiplash-2014/reviews/":   "whiplash-2014",
	}
	for in, want := range tests {
		if got := filmSlug(in); got != want {
			t.Errorf("filmSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScanConfig(t *testing.T) {
	cfg, err := scanConfig([]string{"Zach"}, "", "5,4.5", 3, 50)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Username != "zach" || len(cfg.Stars) != 2 || cfg.Stars[1] != 9 || cfg.MinShared != 3 || cfg.MaxUsers != 50 {
		t.Errorf("got %+v", cfg)
	}

	cfg, err = scanConfig(nil, "a,b,c,d", "5", 2, 0)
	if err != nil || len(cfg.Films) != 4 {
		t.Errorf("--films: got %+v, %v", cfg, err)
	}
}

func TestScanConfigInvalid(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		films     string
		stars     string
		minShared int
		maxUsers  int
	}{
		{"no input", nil, "", "5", 2, 100},
		{"both inputs", []string{"zach"}, "a,b", "5", 2, 100},
		{"too many films", nil, "a,b,c,d,e", "5", 2, 100},
		{"min shared too low", []string{"zach"}, "", "5", 1, 100},
		{"bad stars", []string{"zach"}, "", "5,six", 2, 100},
		{"negative max users", []string{"zach"}, "", "5", 2, -1},
	}
	for _, tt := range tests {
		if _, err := scanConfig(tt.args, tt.films, tt.stars, tt.minShared, tt.maxUsers); err == nil {
			t.Errorf("%s: want an error", tt.name)
		}
	}
}
