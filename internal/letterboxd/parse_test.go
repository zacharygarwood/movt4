package letterboxd

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseFavorites(t *testing.T) {
	films, err := parseFavorites(fixture(t, "profile.html"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Film{
		{"high-and-low", "High and Low (1963)"},
		{"burning-2018", "Burning (2018)"},
		{"my-neighbor-totoro", "My Neighbor Totoro (1988)"},
		{"mulholland-drive", "Mulholland Drive (2001)"},
	}
	if !reflect.DeepEqual(films, want) {
		t.Errorf("got %v, want %v", films, want)
	}
}

func TestParseSearchPage(t *testing.T) {
	page, err := parseSearchPage(fixture(t, "search.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Usernames) != 20 || page.Usernames[0] != "aleksev" {
		t.Errorf("got %d usernames starting %q, want 20 starting \"aleksev\"", len(page.Usernames), page.Usernames[0])
	}
	if page.Cursor != "DwcpA-b50ziRdtXAUFA%3D%3D" {
		t.Errorf("cursor = %q", page.Cursor)
	}
}

func TestParseRatedPage(t *testing.T) {
	page, err := parseRatedPage(fixture(t, "rated.html"))
	if err != nil {
		t.Fatal(err)
	}
	if films := page.Films[10]; len(page.Films) != 1 || len(films) != 5 || films[0] != (Film{"poor-things-2023", "Poor Things (2023)"}) {
		t.Errorf("got %v, want 5 five-star films starting with Poor Things", page.Films)
	}
	if page.NextPath != "/dave/films/rated/5/page/2/" {
		t.Errorf("next = %q", page.NextPath)
	}
}

func TestParseRatedPageLast(t *testing.T) {
	page, err := parseRatedPage(`<ul><li class="griditem"><div data-item-slug="x" data-item-name="X"></div><p><span class="rating -micro rated-9">★★★★½</span></p></li></ul>`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Film{{"x", "X"}}; !reflect.DeepEqual(page.Films[9], want) || page.NextPath != "" {
		t.Errorf("got %+v, want X rated ★★★★½ and no next page", page)
	}
}

func TestParsePosterURL(t *testing.T) {
	url, err := parsePosterURL(fixture(t, "film.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "https://a.ltrbxd.com/resized/film-poster/") || !strings.Contains(url, "the-lighthouse") {
		t.Errorf("url = %q", url)
	}
}

func TestParseFilmSearch(t *testing.T) {
	films, err := parseFilmSearch(budapestResults)
	if err != nil {
		t.Fatal(err)
	}
	want := []Film{
		{"the-grand-budapest-hotel", "The Grand Budapest Hotel (2014)"},
		{"the-making-of-the-grand-budapest-hotel", "The Making of The Grand Budapest Hotel (2014)"},
	}
	if !reflect.DeepEqual(films, want) {
		t.Errorf("got %v, want %v", films, want)
	}
}
