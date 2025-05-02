package store

import (
	"testing"
)

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
