package letterboxd

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
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
	Films    map[Rating][]Film
	NextPath string // e.g. "/dave/films/rated/4-5/page/2/"; empty on the last page
}

// ratedClass is the class that carries a film's rating, e.g. "rated-9" for ★★★★½.
var ratedClass = regexp.MustCompile(`\brated-(\d+)\b`)

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

// parseRatedPage extracts films, grouped by rating, from a
// /<user>/films/rated/<stars>/ page.
func parseRatedPage(html string) (ratedPage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ratedPage{}, err
	}
	page := ratedPage{
		Films:    map[Rating][]Film{},
		NextPath: doc.Find(".pagination a.next").AttrOr("href", ""),
	}
	doc.Find("li.griditem").Each(func(_ int, item *goquery.Selection) {
		poster := item.Find("[data-item-slug]").First()
		match := ratedClass.FindStringSubmatch(item.Find(".rating").AttrOr("class", ""))
		if poster.Length() == 0 || match == nil {
			return
		}
		stars, _ := strconv.Atoi(match[1])
		film := Film{Slug: poster.AttrOr("data-item-slug", ""), Title: poster.AttrOr("data-item-name", "")}
		page.Films[Rating(stars)] = append(page.Films[Rating(stars)], film)
	})
	return page, nil
}

// parseFilmSearch reads films from Letterboxd's film autocomplete JSON, best
// match first.
func parseFilmSearch(body string) ([]Film, error) {
	var results struct {
		Data []struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			ReleaseYear int    `json:"releaseYear"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &results); err != nil {
		return nil, err
	}
	films := make([]Film, len(results.Data))
	for i, r := range results.Data {
		films[i] = Film{Slug: r.Slug, Title: r.Name}
		if r.ReleaseYear != 0 {
			films[i].Title = fmt.Sprintf("%s (%d)", r.Name, r.ReleaseYear)
		}
	}
	return films, nil
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
