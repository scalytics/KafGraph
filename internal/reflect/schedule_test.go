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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleIsDueDaily(t *testing.T) {
	s := Schedule{Enabled: true, Hour: 2, Minute: 0}
	at := time.Date(2026, 3, 3, 2, 0, 0, 0, time.UTC)
	assert.True(t, s.IsDue(at, time.Time{}))
}

func TestScheduleIsDueWrongTime(t *testing.T) {
	s := Schedule{Enabled: true, Hour: 2, Minute: 0}
	at := time.Date(2026, 3, 3, 3, 0, 0, 0, time.UTC)
	assert.False(t, s.IsDue(at, time.Time{}))
}

func TestScheduleIsDueDisabled(t *testing.T) {
	s := Schedule{Enabled: false, Hour: 2, Minute: 0}
	at := time.Date(2026, 3, 3, 2, 0, 0, 0, time.UTC)
	assert.False(t, s.IsDue(at, time.Time{}))
}

func TestScheduleIsDueWeekly(t *testing.T) {
	monday := time.Monday
	s := Schedule{Enabled: true, Hour: 3, Minute: 0, DayOfWeek: &monday}

	// 2026-03-02 is a Monday
	mon := time.Date(2026, 3, 2, 3, 0, 0, 0, time.UTC)
	assert.True(t, s.IsDue(mon, time.Time{}))

	// 2026-03-03 is a Tuesday
	tue := time.Date(2026, 3, 3, 3, 0, 0, 0, time.UTC)
	assert.False(t, s.IsDue(tue, time.Time{}))
}

func TestScheduleIsDueMonthly(t *testing.T) {
	day := 1
	s := Schedule{Enabled: true, Hour: 4, Minute: 0, DayOfMonth: &day}

	first := time.Date(2026, 3, 1, 4, 0, 0, 0, time.UTC)
	assert.True(t, s.IsDue(first, time.Time{}))

	second := time.Date(2026, 3, 2, 4, 0, 0, 0, time.UTC)
	assert.False(t, s.IsDue(second, time.Time{}))
}

func TestScheduleIsDueMinGapDaily(t *testing.T) {
	s := Schedule{Enabled: true, Hour: 2, Minute: 0}
	at := time.Date(2026, 3, 3, 2, 0, 0, 0, time.UTC)
	lastRun := at.Add(-20 * time.Hour) // only 20h ago, needs 23h minimum
	assert.False(t, s.IsDue(at, lastRun))
}

func TestScheduleIsDueMinGapDailyOK(t *testing.T) {
	s := Schedule{Enabled: true, Hour: 2, Minute: 0}
	at := time.Date(2026, 3, 3, 2, 0, 0, 0, time.UTC)
	lastRun := at.Add(-24 * time.Hour) // 24h ago, meets 23h minimum
	assert.True(t, s.IsDue(at, lastRun))
}

func TestScheduleIsDueMinGapWeekly(t *testing.T) {
	monday := time.Monday
	s := Schedule{Enabled: true, Hour: 3, Minute: 0, DayOfWeek: &monday}
	at := time.Date(2026, 3, 2, 3, 0, 0, 0, time.UTC) // Monday
	lastRun := at.Add(-5 * 24 * time.Hour)            // 5 days ago, needs 6d minimum
	assert.False(t, s.IsDue(at, lastRun))
}

func TestParseScheduleDaily(t *testing.T) {
	s, err := ParseSchedule("02:00", "", 0)
	require.NoError(t, err)
	assert.True(t, s.Enabled)
	assert.Equal(t, 2, s.Hour)
	assert.Equal(t, 0, s.Minute)
	assert.Nil(t, s.DayOfWeek)
	assert.Nil(t, s.DayOfMonth)
}

func TestParseScheduleWeekly(t *testing.T) {
	s, err := ParseSchedule("03:30", "Monday", 0)
	require.NoError(t, err)
	assert.Equal(t, 3, s.Hour)
	assert.Equal(t, 30, s.Minute)
	require.NotNil(t, s.DayOfWeek)
	assert.Equal(t, time.Monday, *s.DayOfWeek)
}

func TestParseScheduleMonthly(t *testing.T) {
	s, err := ParseSchedule("04:00", "", 15)
	require.NoError(t, err)
	assert.Equal(t, 4, s.Hour)
	require.NotNil(t, s.DayOfMonth)
	assert.Equal(t, 15, *s.DayOfMonth)
}

func TestParseScheduleInvalidTime(t *testing.T) {
	_, err := ParseSchedule("invalid", "", 0)
	assert.Error(t, err)
}

func TestParseScheduleInvalidHour(t *testing.T) {
	_, err := ParseSchedule("25:00", "", 0)
	assert.Error(t, err)
}

func TestParseScheduleInvalidWeekday(t *testing.T) {
	_, err := ParseSchedule("02:00", "Funday", 0)
	assert.Error(t, err)
}

func TestParseScheduleInvalidDayOfMonth(t *testing.T) {
	_, err := ParseSchedule("02:00", "", 32)
	assert.Error(t, err)
}
