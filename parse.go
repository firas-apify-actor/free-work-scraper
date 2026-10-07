package main

import (
	"regexp"
	"strconv"
)

// The API already returns structured min/max daily rates; parseRate is only the
// fallback for rates written in the description text. It matches only an explicit
// day marker ("€/j", "par jour", "TJM") and returns nil otherwise: never guess.
var (
	rateDayRe = regexp.MustCompile(`(?i)(\d{2,4})(?:\s*(?:€|euros?|eur))?(?:\s*(?:-|–|à|/)\s*(\d{2,4}))?\s*(?:€|euros?|eur)?\s*(?:HT\s*)?(?:/|par\s+)\s*(?:j|jour)s?\b`)
	rateTJMRe = regexp.MustCompile(`(?i)\bTJM\s*(?:cible|max|souhait\w*|indicatif)?\s*[:=]?\s*(?:de\s+)?(\d{2,4})(?:\s*(?:€|euros?)?\s*(?:-|–|à)\s*(\d{2,4}))?`)
)

// parseRate returns (min, max) in EUR/day, or nil, nil when not unambiguous.
func parseRate(s string) (*float64, *float64) {
	for _, re := range []*regexp.Regexp{rateDayRe, rateTJMRe} {
		m := re.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		lo, _ := strconv.ParseFloat(m[1], 64)
		hi := lo
		if m[2] != "" {
			hi, _ = strconv.ParseFloat(m[2], 64)
		}
		if lo > hi {
			lo, hi = hi, lo
		}
		if lo < 100 || hi > 2500 { // not a plausible day rate
			continue
		}
		return &lo, &hi
	}
	return nil, nil
}

func normalizeExperience(s string) string {
	switch s {
	case "junior", "intermediate", "senior", "expert":
		return s
	}
	return "unknown"
}
