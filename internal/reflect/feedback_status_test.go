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

	"github.com/stretchr/testify/assert"
)

func TestFeedbackStatusIsValid(t *testing.T) {
	assert.True(t, FBPending.IsValid())
	assert.True(t, FBNeedsFeedback.IsValid())
	assert.True(t, FBRequested.IsValid())
	assert.True(t, FBReceived.IsValid())
	assert.True(t, FBWaived.IsValid())
}

func TestFeedbackStatusInvalid(t *testing.T) {
	assert.False(t, FeedbackStatus("UNKNOWN").IsValid())
	assert.False(t, FeedbackStatus("").IsValid())
}

func TestFeedbackStatusIsTerminal(t *testing.T) {
	assert.True(t, FBReceived.IsTerminal())
	assert.True(t, FBWaived.IsTerminal())
}

func TestFeedbackStatusNonTerminal(t *testing.T) {
	assert.False(t, FBPending.IsTerminal())
	assert.False(t, FBNeedsFeedback.IsTerminal())
	assert.False(t, FBRequested.IsTerminal())
}

func TestFeedbackStatusStringValues(t *testing.T) {
	assert.Equal(t, "PENDING", string(FBPending))
	assert.Equal(t, "NEEDS_FEEDBACK", string(FBNeedsFeedback))
	assert.Equal(t, "REQUESTED", string(FBRequested))
	assert.Equal(t, "RECEIVED", string(FBReceived))
	assert.Equal(t, "WAIVED", string(FBWaived))
}
