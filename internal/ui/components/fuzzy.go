// Package components provides reusable UI components for awsc.
package components

import (
	"sort"
	"strings"
)

// FuzzyMatch represents a fuzzy match result with scoring.
type FuzzyMatch struct {
	Value string
	Score int // higher = better match
}

// FuzzyMatches sorts matches by score (descending), then alphabetically.
type FuzzyMatches []FuzzyMatch

func (m FuzzyMatches) Len() int      { return len(m) }
func (m FuzzyMatches) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m FuzzyMatches) Less(i, j int) bool {
	if m[i].Score != m[j].Score {
		return m[i].Score > m[j].Score // higher score first
	}
	return m[i].Value < m[j].Value // alphabetical tiebreaker
}

// FuzzyScore calculates how well query matches target.
// Returns -1 if no match, otherwise a score (higher = better).
//
// Scoring:
//   - Exact match: 1000
//   - Prefix match: 500 + length bonus
//   - Contains match: 200 + position bonus
//   - Fuzzy match (all chars in order): 100 + bonuses
//   - No match: -1
func FuzzyScore(query, target string) int {
	if query == "" {
		return 0 // empty query matches everything
	}

	queryLower := strings.ToLower(query)
	targetLower := strings.ToLower(target)

	// Exact match
	if queryLower == targetLower {
		return 1000
	}

	// Prefix match
	if strings.HasPrefix(targetLower, queryLower) {
		return 500 + len(query)
	}

	// Contains match (query is a contiguous substring)
	if idx := strings.Index(targetLower, queryLower); idx >= 0 {
		return 200 + (100 - idx) // earlier position = better
	}

	// Fuzzy match: all query chars appear in order in target
	// First query char must appear at a word boundary (start, or after - or _)
	// This prevents overly broad matches like "us" matching "eu-west-1"
	if !startsAtWordBoundary(queryLower, targetLower) {
		return -1
	}

	score := fuzzyCharMatch(queryLower, targetLower)
	if score > 0 {
		return 100 + score
	}

	return -1
}

// startsAtWordBoundary checks if the first char of query appears at a word
// boundary in target (at index 0, or immediately after - or _).
func startsAtWordBoundary(query, target string) bool {
	if len(query) == 0 || len(target) == 0 {
		return false
	}
	firstChar := query[0]
	for i := 0; i < len(target); i++ {
		if target[i] == firstChar {
			// Check if this position is a word boundary
			if i == 0 || target[i-1] == '-' || target[i-1] == '_' {
				return true
			}
		}
	}
	return false
}

// fuzzyCharMatch checks if all chars in query appear in order in target.
// Returns a bonus score based on consecutive matches and position.
func fuzzyCharMatch(query, target string) int {
	if len(query) == 0 {
		return 0
	}
	if len(query) > len(target) {
		return -1
	}

	qi := 0 // query index
	bonus := 0
	lastMatchIdx := -1

	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if target[ti] == query[qi] {
			// Bonus for consecutive matches
			if lastMatchIdx == ti-1 {
				bonus += 10
			}
			// Bonus for matching at word boundaries (after - or start)
			if ti == 0 || target[ti-1] == '-' || target[ti-1] == '_' {
				bonus += 5
			}
			lastMatchIdx = ti
			qi++
		}
	}

	if qi == len(query) {
		return bonus + 1 // matched all chars
	}
	return -1
}

// FuzzyFilter filters candidates by query and returns sorted matches.
// Only returns candidates with score >= 0.
func FuzzyFilter(query string, candidates []string) []string {
	if query == "" {
		// Return all candidates sorted alphabetically
		result := make([]string, len(candidates))
		copy(result, candidates)
		sort.Strings(result)
		return result
	}

	var matches FuzzyMatches
	for _, c := range candidates {
		score := FuzzyScore(query, c)
		if score >= 0 {
			matches = append(matches, FuzzyMatch{Value: c, Score: score})
		}
	}

	sort.Sort(matches)

	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.Value
	}
	return result
}

// FuzzyBest returns the best matching candidate, or empty string if none match.
func FuzzyBest(query string, candidates []string) string {
	matches := FuzzyFilter(query, candidates)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}
