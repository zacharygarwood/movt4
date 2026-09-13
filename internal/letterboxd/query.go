package letterboxd

import "strings"

// FanQuery builds a member search query for people whose favorite films
// include at least minShared of slugs. Each combination of minShared films
// becomes an AND group, and the groups are joined with OR:
//
//	(fan:a fan:b fan:c) OR (fan:a fan:b fan:d) OR ...
func FanQuery(slugs []string, minShared int) string {
	var groups []string
	for _, combo := range combinations(slugs, minShared) {
		terms := make([]string, len(combo))
		for i, slug := range combo {
			terms[i] = "fan:" + slug
		}
		groups = append(groups, "("+strings.Join(terms, " ")+")")
	}
	return strings.Join(groups, " OR ")
}

// combinations returns every k-element subset of items, preserving order.
func combinations(items []string, k int) [][]string {
	if k == 0 {
		return [][]string{{}}
	}
	if len(items) < k {
		return nil
	}
	head, rest := items[0], items[1:]
	var out [][]string
	for _, combo := range combinations(rest, k-1) {
		out = append(out, append([]string{head}, combo...))
	}
	return append(out, combinations(rest, k)...)
}
