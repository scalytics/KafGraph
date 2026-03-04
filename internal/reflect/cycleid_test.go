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
)

func TestCycleNodeIDDeterministic(t *testing.T) {
	ws := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	id1 := CycleNodeID(CycleDaily, "alice", ws)
	id2 := CycleNodeID(CycleDaily, "alice", ws)
	assert.Equal(t, id1, id2)
}

func TestCycleNodeIDFormat(t *testing.T) {
	ws := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	id := CycleNodeID(CycleDaily, "alice", ws)
	assert.Equal(t, "n:ReflectionCycle:daily:alice:2026-03-03T00:00:00Z", string(id))
}

func TestCycleNodeIDDifferentTypes(t *testing.T) {
	ws := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	daily := CycleNodeID(CycleDaily, "alice", ws)
	weekly := CycleNodeID(CycleWeekly, "alice", ws)
	monthly := CycleNodeID(CycleMonthly, "alice", ws)
	assert.NotEqual(t, daily, weekly)
	assert.NotEqual(t, weekly, monthly)
	assert.NotEqual(t, daily, monthly)
}

func TestCycleNodeIDDifferentAgents(t *testing.T) {
	ws := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	alice := CycleNodeID(CycleDaily, "alice", ws)
	bob := CycleNodeID(CycleDaily, "bob", ws)
	assert.NotEqual(t, alice, bob)
}

func TestSignalNodeIDDeterministic(t *testing.T) {
	ws := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	id1 := SignalNodeID(CycleDaily, "alice", ws, "n:Message:topic:0:42")
	id2 := SignalNodeID(CycleDaily, "alice", ws, "n:Message:topic:0:42")
	assert.Equal(t, id1, id2)
}

func TestSignalNodeIDFormat(t *testing.T) {
	ws := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	id := SignalNodeID(CycleDaily, "alice", ws, "n:Message:topic:0:42")
	assert.Contains(t, string(id), "n:LearningSignal:daily:alice:")
	assert.Contains(t, string(id), "n:Message:topic:0:42")
}

func TestSignalNodeIDDifferentSources(t *testing.T) {
	ws := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	id1 := SignalNodeID(CycleDaily, "alice", ws, "n:Message:topic:0:42")
	id2 := SignalNodeID(CycleDaily, "alice", ws, "n:Message:topic:0:99")
	assert.NotEqual(t, id1, id2)
}

func TestScoreEdgeIDDeterministic(t *testing.T) {
	id1 := ScoreEdgeID("LINKS_TO", "n:Signal:1", "n:Cycle:1")
	id2 := ScoreEdgeID("LINKS_TO", "n:Signal:1", "n:Cycle:1")
	assert.Equal(t, id1, id2)
}

func TestScoreEdgeIDFormat(t *testing.T) {
	id := ScoreEdgeID("LINKS_TO", "n:Signal:1", "n:Cycle:1")
	assert.Contains(t, string(id), "e:LINKS_TO:")
}

func TestDailyWindowStart(t *testing.T) {
	// Afternoon should truncate to midnight
	input := time.Date(2026, 3, 3, 15, 30, 45, 0, time.UTC)
	ws := DailyWindowStart(input)
	assert.Equal(t, time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC), ws)
}

func TestDailyWindowStartAlreadyMidnight(t *testing.T) {
	input := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	ws := DailyWindowStart(input)
	assert.Equal(t, input, ws)
}

func TestWeeklyWindowStartMonday(t *testing.T) {
	// 2026-03-02 is a Monday
	monday := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	ws := WeeklyWindowStart(monday)
	assert.Equal(t, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC), ws)
}

func TestWeeklyWindowStartWednesday(t *testing.T) {
	// 2026-03-04 is a Wednesday → should go back to Monday 2026-03-02
	wed := time.Date(2026, 3, 4, 14, 0, 0, 0, time.UTC)
	ws := WeeklyWindowStart(wed)
	assert.Equal(t, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC), ws)
}

func TestWeeklyWindowStartSunday(t *testing.T) {
	// 2026-03-08 is a Sunday → should go back to Monday 2026-03-02
	sun := time.Date(2026, 3, 8, 14, 0, 0, 0, time.UTC)
	ws := WeeklyWindowStart(sun)
	assert.Equal(t, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC), ws)
}

func TestMonthlyWindowStart(t *testing.T) {
	input := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	ws := MonthlyWindowStart(input)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), ws)
}

func TestMonthlyWindowStartFirstOfMonth(t *testing.T) {
	input := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	ws := MonthlyWindowStart(input)
	assert.Equal(t, input, ws)
}
