// Package export writes scan results as JSON and CSV files.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
	"github.com/zacharygarwood/movt4/internal/scan"
)

// Input describes what a scan matched against.
type Input struct {
	Username  string // empty when films were given directly
	Favorites []letterboxd.Film
	Stars     []letterboxd.Rating
}

// report is the JSON document. It keeps each user's raw ratings so the data
// can be re-analyzed without scanning again.
type report struct {
	GeneratedAt time.Time         `json:"generatedAt"`
	Username    string            `json:"username,omitempty"`
	Favorites   []letterboxd.Film `json:"favorites"`
	Users       []user            `json:"users"`
	Films       map[string]string `json:"films"` // slug → title
}

type user struct {
	Username string              `json:"username"`
	Shared   int                 `json:"shared"`
	Ratings  map[string][]string `json:"ratings"` // stars, e.g. "4.5" → film slugs
}

// WriteJSON writes every matched user with their ratings.
func WriteJSON(w io.Writer, in Input, results *scan.Results, now time.Time) error {
	rep := report{
		GeneratedAt: now.UTC(),
		Username:    in.Username,
		Favorites:   in.Favorites,
		Users:       []user{},
		Films:       map[string]string{},
	}
	for _, u := range results.Users {
		out := user{Username: u.Username, Shared: u.Shared, Ratings: map[string][]string{}}
		for rating, films := range u.Ratings {
			slugs := []string{}
			for _, f := range films {
				slugs = append(slugs, f.Slug)
				rep.Films[f.Slug] = f.Title
			}
			out.Ratings[rating.Stars()] = slugs
		}
		rep.Users = append(rep.Users, out)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// WriteCSV writes one row per film and rating, counting users by how many
// of the Top 4 they share.
func WriteCSV(w io.Writer, in Input, results *scan.Results) error {
	out := csv.NewWriter(w)
	out.Write([]string{"rating", "slug", "title", "shared_4", "shared_3", "shared_2", "total"})
	for _, rating := range in.Stars {
		for _, c := range results.Tally(rating, 0) {
			out.Write([]string{
				rating.Stars(), c.Film.Slug, c.Film.Title,
				strconv.Itoa(c.ByShared[4]), strconv.Itoa(c.ByShared[3]), strconv.Itoa(c.ByShared[2]),
				strconv.Itoa(c.Total),
			})
		}
	}
	out.Flush()
	return out.Error()
}

// Files writes a JSON and a CSV file into dir and returns their paths.
func Files(dir string, in Input, results *scan.Results, now time.Time) (jsonPath, csvPath string, err error) {
	name := in.Username
	if name == "" {
		name = "films"
	}
	base := filepath.Join(dir, fmt.Sprintf("movt4-%s-%s", name, now.Format("20060102-150405")))
	jsonPath, csvPath = base+".json", base+".csv"

	if err := writeFile(jsonPath, func(w io.Writer) error { return WriteJSON(w, in, results, now) }); err != nil {
		return "", "", err
	}
	if err := writeFile(csvPath, func(w io.Writer) error { return WriteCSV(w, in, results) }); err != nil {
		return "", "", err
	}
	return jsonPath, csvPath, nil
}

func writeFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
