package scan

import (
	"cmp"
	"slices"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
)

// User is a matched member and the films they rated.
type User struct {
	Username string
	Shared   int // how many of the Top 4 are among their favorites
	Ratings  map[letterboxd.Rating][]letterboxd.Film
}

// Results collects scanned users. Tallies are computed on demand so the
// rating and shared-tier filters can change at any time.
type Results struct {
	Users []User
}

// FilmCount is how many users gave a film a particular rating.
type FilmCount struct {
	Film     letterboxd.Film
	ByShared [5]int // indexed by how many of the Top 4 those users share
	Total    int
}

func (r *Results) Add(u User) {
	r.Users = append(r.Users, u)
}

// CountUsers returns how many users share at least minShared films.
func (r *Results) CountUsers(minShared int) int {
	n := 0
	for _, u := range r.Users {
		if u.Shared >= minShared {
			n++
		}
	}
	return n
}

// Tally counts how many users sharing at least minShared films gave each
// film the rating, most common first.
func (r *Results) Tally(rating letterboxd.Rating, minShared int) []FilmCount {
	counts := map[string]*FilmCount{}
	for _, u := range r.Users {
		if u.Shared < minShared {
			continue
		}
		for _, film := range u.Ratings[rating] {
			c, ok := counts[film.Slug]
			if !ok {
				c = &FilmCount{Film: film}
				counts[film.Slug] = c
			}
			c.ByShared[u.Shared]++
			c.Total++
		}
	}

	tally := make([]FilmCount, 0, len(counts))
	for _, c := range counts {
		tally = append(tally, *c)
	}
	slices.SortFunc(tally, func(a, b FilmCount) int {
		return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(a.Film.Title, b.Film.Title))
	})
	return tally
}
