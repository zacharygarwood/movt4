package letterboxd

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // posters are JPEGs
	"iter"
	"net/url"
	"strings"
)

// Client reads Letterboxd. Pages behind Cloudflare's challenge go through
// the guarded fetcher; everything else uses the cheaper open fetcher.
type Client struct {
	open    Fetcher
	guarded Fetcher
}

func NewClient(open, guarded Fetcher) *Client {
	return &Client{open: open, guarded: guarded}
}

// Favorites returns a member's Top 4 films.
func (c *Client) Favorites(ctx context.Context, username string) ([]Film, error) {
	body, err := c.open.Get(ctx, baseURL+"/"+url.PathEscape(username)+"/")
	if err != nil {
		return nil, err
	}
	return parseFavorites(body)
}

// SearchFans yields pages of usernames whose favorites include at least
// minShared of slugs. Consecutive pages can repeat a username.
func (c *Client) SearchFans(ctx context.Context, slugs []string, minShared int) iter.Seq2[[]string, error] {
	endpoint := baseURL + "/s/search/members/" + url.PathEscape(FanQuery(slugs, minShared)) + "/"
	return func(yield func([]string, error) bool) {
		cursor := ""
		for {
			pageURL := endpoint
			if cursor != "" {
				pageURL += "?cursor=" + cursor // the cursor arrives URL-encoded
			}
			body, err := c.guarded.Get(ctx, pageURL)
			if err != nil {
				yield(nil, err)
				return
			}
			page, err := parseSearchPage(body)
			if err != nil {
				yield(nil, err)
				return
			}
			if len(page.Usernames) == 0 || !yield(page.Usernames, nil) {
				return
			}
			if page.Cursor == "" || page.Cursor == cursor {
				return
			}
			cursor = page.Cursor
		}
	}
}

// RatedFilms returns the films a member rated from `from` to `to` stars,
// grouped by rating. A range costs one request per 72 films however many
// ratings it spans, so it's much cheaper than asking for each rating.
func (c *Client) RatedFilms(ctx context.Context, username string, from, to Rating) (map[Rating][]Film, error) {
	stars := from.Stars()
	if to != from {
		stars += "-" + to.Stars()
	}
	films := map[Rating][]Film{}
	path := "/" + url.PathEscape(username) + "/films/rated/" + stars + "/"
	for path != "" {
		body, err := c.guarded.Get(ctx, baseURL+path)
		if err != nil {
			return nil, err
		}
		page, err := parseRatedPage(body)
		if err != nil {
			return nil, err
		}
		for rating, rated := range page.Films {
			films[rating] = append(films[rating], rated...)
		}
		path = page.NextPath
	}
	return films, nil
}

// PosterURL returns the URL of a film's poster image.
func (c *Client) PosterURL(ctx context.Context, slug string) (string, error) {
	body, err := c.open.Get(ctx, baseURL+"/film/"+url.PathEscape(slug)+"/")
	if err != nil {
		return "", err
	}
	return parsePosterURL(body)
}

// Poster downloads and decodes a film's poster.
func (c *Client) Poster(ctx context.Context, slug string) (image.Image, error) {
	posterURL, err := c.PosterURL(ctx, slug)
	if err != nil {
		return nil, err
	}
	body, err := c.open.Get(ctx, posterURL)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decoding poster for %s: %w", slug, err)
	}
	return img, nil
}
