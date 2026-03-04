// Copyright 2026 Scalytics, Inc.
// Copyright 2026 Mirko Kämpf
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reflect

import (
	"fmt"
	"strings"
	"time"
)

// Schedule defines when a reflection cycle should fire.
type Schedule struct {
	Enabled    bool
	Hour       int           // 0-23 UTC
	Minute     int           // 0-59
	DayOfWeek  *time.Weekday // nil = daily; set for weekly
	DayOfMonth *int          // nil = any; set for monthly
}

// IsDue returns true if the schedule should fire at the given time,
// considering the last run time to prevent double-firing.
func (s *Schedule) IsDue(now time.Time, lastRun time.Time) bool {
	if !s.Enabled {
		return false
	}

	u := now.UTC()
	if u.Hour() != s.Hour || u.Minute() != s.Minute {
		return false
	}

	if s.DayOfWeek != nil && u.Weekday() != *s.DayOfWeek {
		return false
	}

	if s.DayOfMonth != nil && u.Day() != *s.DayOfMonth {
		return false
	}

	// Minimum gap guard to prevent double-firing
	if !lastRun.IsZero() {
		var minGap time.Duration
		switch {
		case s.DayOfMonth != nil:
			minGap = 27 * 24 * time.Hour
		case s.DayOfWeek != nil:
			minGap = 6 * 24 * time.Hour
		default:
			minGap = 23 * time.Hour
		}
		if now.Sub(lastRun) < minGap {
			return false
		}
	}

	return true
}

// ParseSchedule creates a Schedule from configuration strings.
// timeStr is "HH:MM", dayStr is a weekday name (empty for daily),
// dayOfMonth is the day of month (0 for any).
func ParseSchedule(timeStr, dayStr string, dayOfMonth int) (Schedule, error) {
	s := Schedule{Enabled: true}

	// Parse time
	parts := strings.SplitN(timeStr, ":", 2)
	if len(parts) != 2 {
		return s, fmt.Errorf("invalid time format %q, expected HH:MM", timeStr)
	}

	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return s, fmt.Errorf("invalid hour %q: %w", parts[0], err)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return s, fmt.Errorf("invalid minute %q: %w", parts[1], err)
	}
	if h < 0 || h > 23 {
		return s, fmt.Errorf("hour %d out of range 0-23", h)
	}
	if m < 0 || m > 59 {
		return s, fmt.Errorf("minute %d out of range 0-59", m)
	}
	s.Hour = h
	s.Minute = m

	// Parse weekday
	if dayStr != "" {
		wd, err := parseWeekday(dayStr)
		if err != nil {
			return s, err
		}
		s.DayOfWeek = &wd
	}

	// Parse day of month
	if dayOfMonth > 0 {
		if dayOfMonth > 31 {
			return s, fmt.Errorf("day of month %d out of range 1-31", dayOfMonth)
		}
		s.DayOfMonth = &dayOfMonth
	}

	return s, nil
}

func parseWeekday(name string) (time.Weekday, error) {
	switch strings.ToLower(name) {
	case "sunday":
		return time.Sunday, nil
	case "monday":
		return time.Monday, nil
	case "tuesday":
		return time.Tuesday, nil
	case "wednesday":
		return time.Wednesday, nil
	case "thursday":
		return time.Thursday, nil
	case "friday":
		return time.Friday, nil
	case "saturday":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("unknown weekday %q", name)
	}
}
