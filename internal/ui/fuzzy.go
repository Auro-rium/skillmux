package ui

import (
	"strings"
	"unicode"
)

type fuzzyResult struct {
	Score     int
	Positions []int
}

func fuzzyMatch(text, query string) (fuzzyResult, bool) {
	textRunes := []rune(strings.ToLower(text))
	queryRunes := []rune(strings.ToLower(strings.TrimSpace(query)))
	if len(queryRunes) == 0 {
		return fuzzyResult{}, true
	}

	result := fuzzyResult{Score: 0}
	qi := 0
	last := -2
	for i, r := range textRunes {
		if qi >= len(queryRunes) {
			break
		}
		if r != queryRunes[qi] {
			continue
		}
		result.Positions = append(result.Positions, i)
		result.Score += 10
		if i == last+1 {
			result.Score += 8
		}
		if i == 0 || unicode.IsSpace(textRunes[i-1]) || textRunes[i-1] == '-' || textRunes[i-1] == '_' || textRunes[i-1] == '/' {
			result.Score += 6
		}
		last = i
		qi++
	}
	if qi != len(queryRunes) {
		return fuzzyResult{}, false
	}
	result.Score -= len(textRunes) / 8
	return result, true
}

func highlightMatch(text, query string, normal, match func(string) string) string {
	if strings.TrimSpace(query) == "" {
		return normal(text)
	}
	res, ok := fuzzyMatch(text, query)
	if !ok {
		return normal(text)
	}
	set := map[int]bool{}
	for _, i := range res.Positions {
		set[i] = true
	}
	runes := []rune(text)
	var b strings.Builder
	for i, r := range runes {
		if set[i] {
			b.WriteString(match(string(r)))
		} else {
			b.WriteString(normal(string(r)))
		}
	}
	return b.String()
}
