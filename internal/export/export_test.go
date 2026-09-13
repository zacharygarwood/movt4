package export

import (
	"bytes"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
	"github.com/zacharygarwood/movt4/internal/scan"
)

var update = flag.Bool("update", false, "rewrite golden files")

var (
	parasite = letterboxd.Film{Slug: "parasite-2019", Title: "Parasite (2019)"}
	whiplash = letterboxd.Film{Slug: "whiplash-2014", Title: "Whiplash (2014)"}
)

func sample() (Input, *scan.Results) {
	in := Input{
		Username:  "zach",
		Favorites: []letterboxd.Film{parasite, whiplash},
		Stars:     []letterboxd.Rating{10, 9},
	}
	results := &scan.Results{}
	results.Add(scan.User{Username: "ana", Shared: 2, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {parasite, whiplash}, 9: {}}})
	results.Add(scan.User{Username: "ben", Shared: 2, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {whiplash}, 9: {parasite}}})
	return in, results
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := "testdata/" + name
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch:\n%s\nwant:\n%s", name, got, want)
	}
}

func TestWriteJSON(t *testing.T) {
	in, results := sample()
	var buf bytes.Buffer
	if err := WriteJSON(&buf, in, results, time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	golden(t, "report.json", buf.Bytes())
}

func TestWriteCSV(t *testing.T) {
	in, results := sample()
	var buf bytes.Buffer
	if err := WriteCSV(&buf, in, results); err != nil {
		t.Fatal(err)
	}
	golden(t, "report.csv", buf.Bytes())
}
