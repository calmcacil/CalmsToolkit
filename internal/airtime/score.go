package airtime

import (
	"regexp"
	"strings"
	"unicode"
)

var yearRe = regexp.MustCompile(`\b\d{4}\b`)

type scoredMatch struct {
	Candidate SeriesOrMovie
	Score     int
}

func tokenise(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	var cur strings.Builder
	for _, r := range s {
		if r == '\'' || unicode.IsSpace(r) {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func scoreCandidate(query string, c SeriesOrMovie) int {
	qTokens := tokenise(query)
	titleTokens := tokenise(c.Title)

	if len(qTokens) == 0 {
		return 0
	}

	var score int

	inLibrary := c.Monitored || c.HasFile

	yearStr := ""
	if c.Year > 0 {
		yearStr = itoa(c.Year)
	}

	isYearToken := func(s string) bool { return len(s) == 4 && yearRe.MatchString(s) }

	for _, qt := range qTokens {
		if isYearToken(qt) {
			continue
		}
		found := false
		for _, tt := range titleTokens {
			if strings.Contains(tt, qt) {
				found = true
				break
			}
		}
		if !found {
			return 0
		}
	}

	score += 50

	prefixCount := 0
	for _, qt := range qTokens {
		for _, tt := range titleTokens {
			if strings.HasPrefix(tt, qt) {
				prefixCount++
				break
			}
		}
	}
	score += min(prefixCount, 4) * 20

	qNoApos := strings.ReplaceAll(strings.ToLower(query), "'", "")
	tNoApos := strings.ReplaceAll(strings.ToLower(c.Title), "'", "")
	ti := 0
	for _, r := range qNoApos {
		pos := strings.IndexRune(tNoApos[ti:], r)
		if pos < 0 {
			break
		}
		ti += pos + 1
	}
	if ti > 0 && ti == len(qNoApos) {
		score += 20
	}

	if inLibrary {
		score += 30
	}
	if c.HasFile {
		score += 10
	}

	qYear := yearRe.FindString(query)
	if qYear != "" && yearStr != "" && qYear == yearStr {
		score += 25
	}

	return score
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
