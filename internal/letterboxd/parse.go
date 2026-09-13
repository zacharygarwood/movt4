package letterboxd

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Film is a film as Letterboxd lists it.
type Film struct {
	Slug  string `json:"slug"`
	Title string `json:"title"` // display name with year, e.g. "Parasite (2019)"
}

// searchPage is one page of member search results.
type searchPage struct {
	Usernames []string
	Cursor    string // opaque, already URL-encoded; empty on the last page
}

// ratedPage is one page of a member's films filtered by rating.
type ratedPage struct {
	Films    []Film
	NextPath string // e.g. "/dave/films/rated/5/page/2/"; empty on the last page
}

// parseFavorites extracts the Top 4 from a member's profile page.
func parseFavorites(html string) ([]Film, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	return filmsIn(doc.Find("#favourites [data-item-slug]")), nil
}

// parseSearchPage extracts usernames from a /s/search/members/ fragment.
func parseSearchPage(html string) (searchPage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return searchPage{}, err
	}
	var page searchPage
	doc.Find("li.search-result.-person a.name").Each(func(_ int, a *goquery.Selection) {
		if username := strings.Trim(a.AttrOr("href", ""), "/"); username != "" {
			page.Usernames = append(page.Usernames, username)
		}
	})
	page.Cursor = doc.Find("ul.results").AttrOr("data-cursor", "")
	return page, nil
}

// parseRatedPage extracts films from a /<user>/films/rated/<stars>/ page.
func parseRatedPage(html string) (ratedPage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ratedPage{}, err
	}
	return ratedPage{
		Films:    filmsIn(doc.Find("li.griditem [data-item-slug]")),
		NextPath: doc.Find(".pagination a.next").AttrOr("href", ""),
	}, nil
}

// parsePosterURL reads the poster image URL from a film page's JSON-LD.
func parsePosterURL(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}
	script := doc.Find(`script[type="application/ld+json"]`).First().Text()
	// The JSON is wrapped in /* <![CDATA[ */ ... /* ]]> */ comments.
	start, end := strings.Index(script, "{"), strings.LastIndex(script, "}")
	if start < 0 || end < start {
		return "", errors.New("film page has no structured data")
	}
	var data struct {
		Image string `json:"image"`
	}
	if err := json.Unmarshal([]byte(script[start:end+1]), &data); err != nil {
		return "", err
	}
	if data.Image == "" {
		return "", errors.New("film page has no poster")
	}
	return data.Image, nil
}

func filmsIn(sel *goquery.Selection) []Film {
	var films []Film
	sel.Each(func(_ int, s *goquery.Selection) {
		films = append(films, Film{Slug: s.AttrOr("data-item-slug", ""), Title: s.AttrOr("data-item-name", "")})
	})
	return films
}
