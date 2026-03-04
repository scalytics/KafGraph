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

// FeedbackStatus represents the state of human feedback on a reflection cycle.
// State machine: PENDING → NEEDS_FEEDBACK → REQUESTED → RECEIVED | WAIVED
type FeedbackStatus string

// Feedback status constants for the reflection cycle state machine.
const (
	FBPending       FeedbackStatus = "PENDING"
	FBNeedsFeedback FeedbackStatus = "NEEDS_FEEDBACK"
	FBRequested     FeedbackStatus = "REQUESTED"
	FBReceived      FeedbackStatus = "RECEIVED"
	FBWaived        FeedbackStatus = "WAIVED"
)

// IsValid returns true if the status is a known feedback status.
func (fs FeedbackStatus) IsValid() bool {
	switch fs {
	case FBPending, FBNeedsFeedback, FBRequested, FBReceived, FBWaived:
		return true
	}
	return false
}

// IsTerminal returns true if the status is a terminal state (RECEIVED or WAIVED).
func (fs FeedbackStatus) IsTerminal() bool {
	return fs == FBReceived || fs == FBWaived
}
