package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/zacharygarwood/movt4/internal/scan"
)

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "movt4"
	return v
}

func (m Model) render() string {
	if m.width == 0 {
		return ""
	}
	width, height := m.width-4, m.height-2 // inside the page padding
	top := lipgloss.JoinVertical(lipgloss.Left, m.header(width), "", m.steps(width), "", dividerStyle.Render(strings.Repeat("─", width)), "")
	footer := m.footer()
	chart := m.chart(width, height-lipgloss.Height(top)-lipgloss.Height(footer)-1)
	page := lipgloss.JoinVertical(lipgloss.Left, top, chart, "", footer)
	return lipgloss.NewStyle().Padding(1, 2).Render(page)
}

func (m Model) header(width int) string {
	logo := logoStyle.Render("movt4")
	var titles []string
	for _, f := range m.favorites {
		titles = append(titles, f.Title)
	}
	subtitle := "matching your Top 4"
	if len(titles) > 0 {
		subtitle = strings.Join(titles, " · ")
	}
	return logo + "  " + mutedStyle.Render(ansi.Truncate(subtitle, width-lipgloss.Width(logo)-2, "…"))
}

// stepLabelWidth is the column where step details start.
const stepLabelWidth = 36

func (m Model) steps(width int) string {
	favorites := "Read your Top 4"
	if m.cfg.Scan.Username != "" {
		favorites = "Read " + m.cfg.Scan.Username + "'s Top 4"
	}
	lines := []string{
		m.step(scan.StageConnect, "Start Chromium", ""),
		m.step(scan.StageFavorites, favorites, ""),
		m.step(scan.StageSearch, "Find people who share your taste", m.searchDetail()),
		m.step(scan.StageRatings, "Fetch their ratings", m.ratingsDetail(width-stepLabelWidth-2)),
	}
	if m.err != nil {
		lines = append(lines, "  "+errorStyle.Width(width-2).Render(m.err.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m Model) step(stage scan.Stage, label, detail string) string {
	icon, style := mutedStyle.Render("○"), mutedStyle
	switch {
	case m.err != nil && stage == m.stage:
		icon, style = errorStyle.Render("✗"), textStyle
	case stage < m.stage || (m.done && m.err == nil):
		icon, style = doneStyle.Render("✓"), textStyle
	case stage == m.stage && !m.done:
		icon, style = m.spinner.View(), boldStyle
	}
	return icon + " " + style.Width(stepLabelWidth).Render(ansi.Truncate(label, stepLabelWidth-1, "…")) + detail
}

func (m Model) totalMatches() int {
	total := 0
	for _, n := range m.matches {
		total += n
	}
	return total
}

func (m Model) searchDetail() string {
	if m.stage < scan.StageSearch {
		return ""
	}
	var tiers []string
	for shared := m.topShared(); shared >= m.cfg.Scan.MinShared; shared-- {
		tiers = append(tiers, mutedStyle.Render(fmt.Sprintf("%d/%d ▸ ", shared, m.topShared()))+countStyle.Render(strconv.Itoa(m.matches[shared])))
	}
	detail := strings.Join(tiers, "   ")
	if max := m.cfg.Scan.MaxUsers; max > 0 && m.totalMatches() >= max {
		detail += mutedStyle.Render(fmt.Sprintf("   (limit %d)", max))
	}
	return detail
}

// ratingsDetail fits the progress bar into space columns, leaving room for
// the counts after it.
func (m Model) ratingsDetail(space int) string {
	if m.stage < scan.StageRatings {
		return ""
	}
	processed, total := m.scanned+m.failed, m.totalMatches()
	bar := m.progress
	bar.SetWidth(min(28, max(8, space-14)))
	detail := bar.ViewAs(float64(processed)/float64(max(total, 1))) +
		countStyle.Render(fmt.Sprintf("  %d", processed)) + mutedStyle.Render(fmt.Sprintf("/%d", total))
	if m.failed > 0 {
		detail += mutedStyle.Render(fmt.Sprintf(" · %d skipped", m.failed))
	}
	if m.done && m.err == nil {
		detail += mutedStyle.Render(" · " + m.elapsed.Round(time.Second).String())
	}
	return detail
}

func (m Model) chart(width, height int) string {
	people := m.results.CountUsers(m.minShared)
	header := mutedStyle.Render("‹ ") + starStyle.Render(m.rating().String()) + mutedStyle.Render(" ›") +
		mutedStyle.Render("   shared ≥ ") + accentStyle.Render(strconv.Itoa(m.minShared)) +
		mutedStyle.Render(fmt.Sprintf(" of %d   %d %s", m.topShared(), people, plural(people, "person", "people")))

	tally := m.tally()
	rows := height - 2
	if len(tally) == 0 || rows < 1 {
		empty := "Waiting for ratings…"
		if m.done {
			empty = "Nobody here gave a film this rating."
		}
		return header + "\n\n" + mutedStyle.Render(empty)
	}

	panelWidth := posterWidth + 2 // plus the border
	if m.cfg.Poster == nil || width < 90 || rows < posterHeight+2 {
		return header + "\n\n" + m.bars(tally, width, rows)
	}
	bars := m.bars(tally, width-panelWidth-3, rows)
	return header + "\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, bars, "   ", m.posterPanel())
}

func (m Model) posterPanel() string {
	film, _ := m.selectedFilm()
	p, requested := m.posters[film.Slug]
	art := p.art
	if art == "" {
		message := "Loading poster…"
		if p.failed {
			message = "No poster"
		} else if !requested {
			message = ""
		}
		art = mutedStyle.Width(posterWidth).Height(posterHeight).
			Align(lipgloss.Center, lipgloss.Center).Render(message)
	}
	return posterBorder.Render(art)
}

// bars renders one row per film, scrolled to keep the selection visible.
func (m Model) bars(tally []scan.FilmCount, width, rows int) string {
	selected := min(m.selected, len(tally)-1)
	first := max(0, selected-rows+1)
	last := min(len(tally), first+rows)

	titleWidth := min(32, width/3)
	countWidth := len(strconv.Itoa(tally[0].Total))
	barWidth := max(1, width-2-titleWidth-1-1-countWidth)

	var lines []string
	for i := first; i < last; i++ {
		c := tally[i]
		marker, title, bar := "  ", textStyle, barStyle
		if i == selected {
			marker, title, bar = activeStyle.Render("▸ "), boldStyle, selectedBar
		}
		name := title.Width(titleWidth).Render(ansi.Truncate(c.Film.Title, titleWidth-1, "…"))
		fill := bar.Width(barWidth).Render(barOf(c.Total, tally[0].Total, barWidth))
		count := countStyle.Render(fmt.Sprintf("%*d", countWidth, c.Total))
		lines = append(lines, marker+name+" "+fill+" "+count)
	}
	return strings.Join(lines, "\n")
}

// barOf draws value relative to max using eighth blocks for smooth ends.
func barOf(value, max, width int) string {
	eighths := value * width * 8 / max
	bar := strings.Repeat("█", eighths/8)
	if rem := eighths % 8; rem > 0 {
		bar += string([]rune("▏▎▍▌▋▊▉")[rem-1])
	}
	return bar
}

func (m Model) footer() string {
	keys := [][2]string{{"←/→", "rating"}, {"tab", "shared"}, {"↑/↓", "select"}, {"e", "export"}, {"q", "quit"}}
	var help []string
	for _, k := range keys {
		help = append(help, keyStyle.Render(k[0])+" "+helpTextStyle.Render(k[1]))
	}
	footer := strings.Join(help, helpTextStyle.Render("  ·  "))
	if m.notice != "" {
		footer = m.notice + "\n" + footer
	}
	return footer
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
