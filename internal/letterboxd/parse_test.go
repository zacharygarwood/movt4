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
	if len(page.Films) != 5 || page.Films[0] != (Film{"poor-things-2023", "Poor Things (2023)"}) {
		t.Errorf("got %v", page.Films)
	}
	if page.NextPath != "/dave/films/rated/5/page/2/" {
		t.Errorf("next = %q", page.NextPath)
	}
}

func TestParseRatedPageLast(t *testing.T) {
	page, err := parseRatedPage(`<ul><li class="griditem"><div data-item-slug="x" data-item-name="X"></div></li></ul>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Films) != 1 || page.NextPath != "" {
		t.Errorf("got %+v, want one film and no next page", page)
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
