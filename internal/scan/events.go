package scan

import "github.com/zacharygarwood/movt4/internal/letterboxd"

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

// Finished is the last event. Err is nil when the scan completed.
type Finished struct{ Err error }

func (StageStarted) isEvent()    {}
func (FavoritesLoaded) isEvent() {}
func (MatchesFound) isEvent()    {}
func (UserScanned) isEvent()     {}
func (UserFailed) isEvent()      {}
func (Finished) isEvent()        {}
