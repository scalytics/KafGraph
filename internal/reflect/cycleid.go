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
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// CycleNodeID returns a deterministic node ID for a reflection cycle.
func CycleNodeID(ct CycleType, agentID string, windowStart time.Time) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("n:ReflectionCycle:%s:%s:%s",
		ct, agentID, windowStart.UTC().Format(time.RFC3339)))
}

// SignalNodeID returns a deterministic node ID for a learning signal
// within a reflection cycle.
func SignalNodeID(ct CycleType, agentID string, windowStart time.Time, sourceID graph.NodeID) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("n:LearningSignal:%s:%s:%s:%s",
		ct, agentID, windowStart.UTC().Format(time.RFC3339), sourceID))
}

// ScoreEdgeID returns a deterministic edge ID from label and endpoints.
func ScoreEdgeID(label string, from, to graph.NodeID) graph.EdgeID {
	h := sha256.Sum256(fmt.Appendf(nil, "%s:%s", from, to))
	return graph.EdgeID(fmt.Sprintf("e:%s:%x", label, h[:8]))
}

// DailyWindowStart truncates t to midnight UTC of that day.
func DailyWindowStart(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// WeeklyWindowStart truncates t to midnight UTC of the most recent Monday.
func WeeklyWindowStart(t time.Time) time.Time {
	u := t.UTC()
	y, m, d := u.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	weekday := day.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	return day.AddDate(0, 0, -int(weekday-time.Monday))
}

// MonthlyWindowStart truncates t to midnight UTC of the 1st of the month.
func MonthlyWindowStart(t time.Time) time.Time {
	y, m, _ := t.UTC().Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}
