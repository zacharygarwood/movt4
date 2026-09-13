package letterboxd

import (
	"strings"
	"testing"
)

func TestFanQuery(t *testing.T) {
	slugs := []string{"a", "b", "c", "d"}

	got := FanQuery(slugs, 3)
	want := "(fan:a fan:b fan:c) OR (fan:a fan:b fan:d) OR (fan:a fan:c fan:d) OR (fan:b fan:c fan:d)"
	if got != want {
		t.Errorf("FanQuery(3) =\n%s\nwant\n%s", got, want)
	}

	for minShared, groups := range map[int]int{2: 6, 3: 4, 4: 1} {
		if n := strings.Count(FanQuery(slugs, minShared), "("); n != groups {
			t.Errorf("FanQuery(%d) has %d groups, want %d", minShared, n, groups)
		}
	}
}
