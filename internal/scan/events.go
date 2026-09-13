package scan

import (
	"time"

	"github.com/zacharygarwood/movt4/internal/letterboxd"
)

// Stage is a step of the scan, in the order they run.
type Stage int

const (
	StageConnect   Stage = iota // starting the browser
	StageFavorites              // reading the Top 4
	StageSearch                 // finding members who share it
	StageRatings                // fetching matched members' ratings
)

// Event reports scan progress. Every event is one of the types below.
type Event interface{ isEvent() }

// StageStarted marks the beginning of a stage.
type StageStarted struct{ Stage Stage }

// FavoritesLoaded carries the Top 4 being matched against.
type FavoritesLoaded struct{ Films []letterboxd.Film }

// MatchesFound carries newly matched usernames that share exactly Shared of
// the Top 4. It is sent once per search page.
type MatchesFound struct {
	Shared    int
	Usernames []string
}

// UserScanned carries a matched member's ratings.
type UserScanned struct{ User User }

// UserFailed reports a member whose ratings could not be fetched. The scan
// carries on without them.
type UserFailed struct {
	Username string
	Err      error
}

// Blocked reports that Cloudflare is blocking requests and the blocked
// request will be tried again at RetryAt.
type Blocked struct{ RetryAt time.Time }

// Finished is the last event. Err is nil when the scan completed, including
// when Letterboxd kept blocking it and it finished early; Unscanned counts
// the matches it didn't get to.
type Finished struct {
	Err       error
	Unscanned int
}

func (StageStarted) isEvent()    {}
func (FavoritesLoaded) isEvent() {}
func (MatchesFound) isEvent()    {}
func (UserScanned) isEvent()     {}
func (UserFailed) isEvent()      {}
func (Blocked) isEvent()         {}
func (Finished) isEvent()        {}
