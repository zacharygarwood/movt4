package tui

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/zacharygarwood/movt4/internal/scan"
)

// Layout, in columns.
const (
	stepLabelWidth = 36 // where step details start
	posterGap      = 3  // between the page and the poster
	minPageWidth   = 70 // left for the steps and chart beside the poster
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
	panel := m.posterPanel(width, height)
	if panel != "" {
		width -= lipgloss.Width(panel) + posterGap
	}
	top := lipgloss.JoinVertical(lipgloss.Left, m.header(width), "", m.steps(width), "", dividerStyle.Render(strings.Repeat("─", width)), "")
	footer := m.footer(width)
	chart := m.chart(width, height-lipgloss.Height(top)-lipgloss.Height(footer)-1)
	page := lipgloss.JoinVertical(lipgloss.Left, top, chart, "", footer)
	if panel != "" {
		page = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(width).Render(page), strings.Repeat(" ", posterGap), panel)
	}
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
	if !m.retryAt.IsZero() {
		wait := max(time.Until(m.retryAt).Round(time.Second), 0)
		lines = append(lines, "  "+mutedStyle.Render("Letterboxd asked us to slow down · resuming in "+wait.String()))
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
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
	if m.unscanned > 0 {
		detail += mutedStyle.Render(fmt.Sprintf(" · stopped early, %d not scanned", m.unscanned))
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
	if m.hideFavorites {
		header += mutedStyle.Render("   Top 4 hidden")
	}

	tally := m.tally()
	rows := height - 2
	if len(tally) == 0 || rows < 1 {
		empty := "Waiting for ratings…"
		if m.done {
			empty = "Nobody here gave a film this rating."
		}
		return header + "\n\n" + mutedStyle.Render(empty)
	}
	return header + "\n\n" + m.bars(tally, width, rows)
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

// barOf draws value relative to largest in up to width cells. Bars fill the
// lower three quarters of their row, so the gap above each keeps neighbors
// distinct. That block has no partial-width forms, so a bar is rounded to
// whole cells and never drawn shorter than one.
func barOf(value, largest, width int) string {
	return strings.Repeat("▆", max(1, (value*width+largest/2)/largest))
}

// posterPanel shows the selected film's poster beside the page, with its
// title underneath. It takes up to half of width, and as much of height as
// that allows, while leaving minPageWidth columns for the page. It's empty
// when there's no room or no film is selected.
func (m Model) posterPanel(width, height int) string {
	film, ok := m.selectedFilm()
	if m.cfg.Poster == nil || !ok {
		return ""
	}
	// The border takes two columns and two rows, and the caption one row.
	width, height = posterSize(min(width/2, width-minPageWidth-posterGap)-2, height-3)
	if height < minPosterHeight {
		return ""
	}

	p, requested := m.posters[film.Slug]
	var art string
	if p.img != nil {
		art = m.drawPoster(film.Slug, p.img, width, height)
	} else {
		message := ""
		switch {
		case p.failed:
			message = "No poster"
		case requested:
			message = "Loading poster…"
		}
		art = mutedStyle.Width(width).Height(height).Align(lipgloss.Center, lipgloss.Center).Render(message)
	}
	caption := boldStyle.Width(width + 2).Align(lipgloss.Center).Render(ansi.Truncate(film.Title, width+2, "…"))
	return posterBorder.Render(art) + "\n" + caption
}

// drawPoster renders a poster, reusing the last rendering when the poster and
// its size haven't changed.
func (m Model) drawPoster(slug string, img image.Image, width, height int) string {
	if a := m.art; a.slug != slug || a.width != width || a.height != height {
		*a = posterArt{slug: slug, width: width, height: height, text: renderPoster(img, width, height)}
	}
	return m.art.text
}

func (m Model) footer(width int) string {
	top4 := "hide top 4"
	if m.hideFavorites {
		top4 = "show top 4"
	}
	keys := [][2]string{{"←/→", "rating"}, {"tab", "shared"}, {"f", top4}, {"↑/↓", "select"}, {"e", "export"}, {"q", "quit"}}
	var help []string
	for _, k := range keys {
		help = append(help, keyStyle.Render(k[0])+" "+helpTextStyle.Render(k[1]))
	}
	// Wrap the help onto more lines rather than cutting keys off.
	separator := helpTextStyle.Render("  ·  ")
	lines := []string{help[0]}
	for _, item := range help[1:] {
		if last := len(lines) - 1; lipgloss.Width(lines[last]+separator+item) <= width {
			lines[last] += separator + item
		} else {
			lines = append(lines, item)
		}
	}
	footer := strings.Join(lines, "\n")
	if m.notice != "" {
		footer = ansi.Truncate(m.notice, width, "…") + "\n" + footer
	}
	return footer
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
