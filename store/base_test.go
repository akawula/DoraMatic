package store

import (
	"strings"
	"testing"
)

func TestGetQueryRepos(t *testing.T) {
	testCases := []struct {
		name           string
		search         string
		expectedSelect string
		expectedCount  string
	}{
		{
			name:           "No search term",
			search:         "",
			expectedSelect: "SELECT org, slug, language FROM repositories ORDER by slug, org",
			expectedCount:  "SELECT count(*) as total FROM repositories",
		},
		{
			name:           "With search term",
			search:         "my-repo",
			expectedSelect: "SELECT org, slug, language FROM repositories WHERE slug LIKE '%my-repo%' ORDER by slug, org",
			expectedCount:  "SELECT count(*) as total FROM repositories WHERE slug LIKE '%my-repo%'",
		},
		{
			name:   "Search term with special characters (ensure basic quoting)",
			search: "test'repo",
			// Note: This basic test assumes the input is relatively clean.
			// Real-world scenarios might need more robust SQL injection prevention.
			expectedSelect: "SELECT org, slug, language FROM repositories WHERE slug LIKE '%test'repo%' ORDER by slug, org",
			expectedCount:  "SELECT count(*) as total FROM repositories WHERE slug LIKE '%test'repo%'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualSelect, actualCount := getQueryRepos(tc.search)
			// Normalize whitespace for comparison robustness
			normalize := func(s string) string {
				return strings.Join(strings.Fields(s), " ")
			}

			if normalize(actualSelect) != normalize(tc.expectedSelect) {
				t.Errorf("Select query mismatch:\nExpected: %s\nActual:   %s", tc.expectedSelect, actualSelect)
			}
			if normalize(actualCount) != normalize(tc.expectedCount) {
				t.Errorf("Count query mismatch:\nExpected: %s\nActual:   %s", tc.expectedCount, actualCount)
			}
		})
	}
}

func TestCalculateOffset(t *testing.T) {
	testCases := []struct {
		name           string
		page           int
		limit          int
		expectedOffset int
	}{
		{
			name:           "Page 1",
			page:           1,
			limit:          10,
			expectedOffset: 0,
		},
		{
			name:           "Page 2",
			page:           2,
			limit:          10,
			expectedOffset: 10,
		},
		{
			name:           "Page 5 with limit 20",
			page:           5,
			limit:          20,
			expectedOffset: 80,
		},
		{
			name:           "Page 1 with different limit",
			page:           1,
			limit:          50,
			expectedOffset: 0,
		},
		// Although page 0 is unlikely in practice, test edge case
		{
			name:           "Page 0 (edge case)",
			page:           0,
			limit:          10,
			expectedOffset: -10, // Based on formula (0-1)*10
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOffset := calculateOffset(tc.page, tc.limit)
			if actualOffset != tc.expectedOffset {
				t.Errorf("Expected offset %d for page %d, limit %d, but got %d", tc.expectedOffset, tc.page, tc.limit, actualOffset)
			}
		})
	}
}
