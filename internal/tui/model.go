// Package tui runs a scan in the terminal: it shows each step as it runs and
// charts what matched members rated, updating as their ratings arrive.
package tui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/zacharygarwood/movt4/internal/export"
	"github.com/zacharygarwood/movt4/internal/letterboxd"
	"github.com/zacharygarwood/movt4/internal/scan"
)

// Config is what the UI needs to run a scan.
type Config struct {
	Scan scan.Config
	Dial scan.Dialer
}

// Model is the Bubble Tea model for a scan.
type Model struct {
	ctx    context.Context
	cfg    Config
	events <-chan scan.Event
	start  time.Time

	// Scan progress, updated from events.
	stage     scan.Stage
	done      bool
	err       error
	elapsed   time.Duration
	favorites []letterboxd.Film
	matches   [5]int // members found, by how many favorites they share
	scanned   int
	failed    int
	results   scan.Results

	// Chart controls.
	star      int // index into cfg.Scan.Stars
	minShared int
	selected  int
	notice    string // result of the last export

	width, height int
	spinner       spinner.Model
	progress      progress.Model
}

type (
	startedMsg  struct{ events <-chan scan.Event }
	eventMsg    struct{ event scan.Event }
	exportedMsg struct {
		jsonPath, csvPath string
		err               error
	}
)

// New returns a model that starts the scan when the program runs. Cancelling
// ctx stops the scan.
func New(ctx context.Context, cfg Config) Model {
	return Model{
		ctx:       ctx,
		cfg:       cfg,
		start:     time.Now(),
		minShared: cfg.Scan.MinShared,
		spinner:   spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(activeStyle)),
		progress:  progress.New(progress.WithColors(orange, green), progress.WithoutPercentage(), progress.WithWidth(28)),
	}
}

func (m Model) Init() tea.Cmd {
	start := func() tea.Msg { return startedMsg{scan.Run(m.ctx, m.cfg.Scan, m.cfg.Dial)} }
	return tea.Batch(start, m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case spinner.TickMsg:
		if m.done {
			return m, nil // let the spinner stop
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case startedMsg:
		m.events = msg.events
		return m, waitForEvent(m.events)
	case eventMsg:
		m.apply(msg.event)
		return m, waitForEvent(m.events)
	case exportedMsg:
		if msg.err != nil {
			m.notice = errorStyle.Render("Export failed: " + msg.err.Error())
		} else {
			m.notice = doneStyle.Render("✓ Saved ") + textStyle.Render(msg.jsonPath+" and "+msg.csvPath)
		}
	}
	return m, nil
}

// waitForEvent delivers the next scan event as a message.
func waitForEvent(events <-chan scan.Event) tea.Cmd {
	return func() tea.Msg {
		if e, ok := <-events; ok {
			return eventMsg{e}
		}
		return nil
	}
}

func (m *Model) apply(e scan.Event) {
	switch e := e.(type) {
	case scan.StageStarted:
		m.stage = e.Stage
	case scan.FavoritesLoaded:
		m.favorites = e.Films
	case scan.MatchesFound:
		m.matches[e.Shared] += len(e.Usernames)
	case scan.UserScanned:
		m.results.Add(e.User)
		m.scanned++
	case scan.UserFailed:
		m.failed++
	case scan.Finished:
		m.done, m.err, m.elapsed = true, e.Err, time.Since(m.start)
	}
}

func (m Model) handleKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "left", "h":
		if m.star > 0 {
			m.star, m.selected = m.star-1, 0
		}
	case "right", "l":
		if m.star < len(m.cfg.Scan.Stars)-1 {
			m.star, m.selected = m.star+1, 0
		}
	case "tab":
		if m.minShared++; m.minShared > m.topShared() {
			m.minShared = m.cfg.Scan.MinShared
		}
		m.selected = 0
	case "up", "k":
		m.selected = max(m.selected-1, 0)
	case "down", "j":
		m.selected = min(m.selected+1, max(len(m.tally())-1, 0))
	case "e":
		return m, m.export()
	}
	return m, nil
}

// topShared is the most favorites a member can share with the Top 4.
func (m Model) topShared() int {
	if len(m.favorites) == 0 {
		return 4
	}
	return len(m.favorites)
}

func (m Model) rating() letterboxd.Rating {
	return m.cfg.Scan.Stars[m.star]
}

func (m Model) tally() []scan.FilmCount {
	return m.results.Tally(m.rating(), m.minShared)
}

// export writes whatever has been scanned so far, so partial results can be
// saved before the scan finishes.
func (m Model) export() tea.Cmd {
	in := export.Input{Username: m.cfg.Scan.Username, Favorites: m.favorites, Stars: m.cfg.Scan.Stars}
	results := m.results
	return func() tea.Msg {
		jsonPath, csvPath, err := export.Files(".", in, &results, time.Now())
		return exportedMsg{jsonPath, csvPath, err}
	}
}
