package scan

import (
	"testing"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
)

func TestTally(t *testing.T) {
	var r Results
	r.Add(User{Username: "a", Shared: 4, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {parasite, whiplash}}})
	r.Add(User{Username: "b", Shared: 3, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {whiplash}}})
	r.Add(User{Username: "c", Shared: 2, Ratings: map[letterboxd.Rating][]letterboxd.Film{10: {whiplash}, 9: {parasite}}})

	tally := r.Tally(10, 2)
	if len(tally) != 2 || tally[0].Film != whiplash || tally[0].Total != 3 || tally[1].Total != 1 {
		t.Fatalf("Tally(10, 2) = %+v", tally)
	}
	if tally[0].ByShared != [5]int{0, 0, 1, 1, 1} {
		t.Errorf("ByShared = %v", tally[0].ByShared)
	}

	if tally := r.Tally(10, 4); len(tally) != 2 || tally[0].Film != parasite {
		t.Errorf("Tally(10, 4) = %+v, want parasite first by title", tally)
	}
	if n := r.CountUsers(3); n != 2 {
		t.Errorf("CountUsers(3) = %d, want 2", n)
	}
}
