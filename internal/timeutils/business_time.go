package timeutils

import (
	"time"
)

// CalculateBusinessSeconds calculates the duration in seconds between two timestamps,
// excluding Saturdays and Sundays. It assumes full 24-hour weekdays are counted.
// Timestamps are processed based on their existing Location.
func CalculateBusinessSeconds(start, end time.Time) float64 {
	// Ensure start is not after end, and neither is zero.
	if start.IsZero() || end.IsZero() || start.After(end) {
		return 0
	}

	var totalBusinessSeconds float64
	current := start

	// Iterate through the time period, segment by segment.
	// Each segment is either up to the end of the current day, or up to the 'end' timestamp if sooner.
	for current.Before(end) {
		// Determine the end of the current segment.
		// It's either the end of the current day or the overall 'end' timestamp, whichever comes first.
		endOfCurrentDay := time.Date(current.Year(), current.Month(), current.Day()+1, 0, 0, 0, 0, current.Location())
		segmentEnd := endOfCurrentDay
		if end.Before(endOfCurrentDay) {
			segmentEnd = end
		}

		// If the current time (start of the segment) is a weekday, add the duration of this segment.
		weekday := current.Weekday()
		if weekday != time.Saturday && weekday != time.Sunday {
			totalBusinessSeconds += segmentEnd.Sub(current).Seconds()
		}

		// Move to the start of the next segment.
		current = segmentEnd
	}

	return totalBusinessSeconds
}
