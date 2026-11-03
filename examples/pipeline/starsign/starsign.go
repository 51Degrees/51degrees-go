/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

// Package starsign contains logic shared by the Pipeline examples, which all
// determine a star sign from a date of birth in different ways.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/required-examples.md
package starsign

import (
	"fmt"
	"time"
)

// Range describes the inclusive date range for a star sign, ignoring year.
type Range struct {
	Name       string
	StartMonth time.Month
	StartDay   int
	EndMonth   time.Month
	EndDay     int
}

// DefaultRanges are the conventional Western zodiac date ranges.
var DefaultRanges = []Range{
	{"Capricorn", time.December, 22, time.January, 19},
	{"Aquarius", time.January, 20, time.February, 18},
	{"Pisces", time.February, 19, time.March, 20},
	{"Aries", time.March, 21, time.April, 19},
	{"Taurus", time.April, 20, time.May, 20},
	{"Gemini", time.May, 21, time.June, 20},
	{"Cancer", time.June, 21, time.July, 22},
	{"Leo", time.July, 23, time.August, 22},
	{"Virgo", time.August, 23, time.September, 22},
	{"Libra", time.September, 23, time.October, 22},
	{"Scorpio", time.October, 23, time.November, 21},
	{"Sagittarius", time.November, 22, time.December, 21},
}

// SignForRanges returns the star sign for the given month and day using the
// supplied ranges, or "Unknown" if none match.
func SignForRanges(ranges []Range, month time.Month, day int) string {
	for _, r := range ranges {
		if inRange(r, month, day) {
			return r.Name
		}
	}
	return "Unknown"
}

// SignForDate returns the star sign for a date using the default ranges.
func SignForDate(month time.Month, day int) string {
	return SignForRanges(DefaultRanges, month, day)
}

func inRange(r Range, month time.Month, day int) bool {
	start := monthDay(r.StartMonth, r.StartDay)
	end := monthDay(r.EndMonth, r.EndDay)
	value := monthDay(month, day)
	if start <= end {
		return value >= start && value <= end
	}
	// Range wraps across the year boundary (e.g. Capricorn).
	return value >= start || value <= end
}

func monthDay(m time.Month, d int) int { return int(m)*100 + d }

// ParseDate parses a date of birth in the form YYYY-MM-DD.
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date of birth %q (expected YYYY-MM-DD): %w", s, err)
	}
	return t, nil
}
