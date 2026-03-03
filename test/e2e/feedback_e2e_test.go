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

//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/ingest"
	reflectpkg "github.com/scalytics/kafgraph/internal/reflect"
	"github.com/scalytics/kafgraph/internal/storage"
)

// TestE2EFeedbackLoop exercises the full feedback loop:
// 1. Run a reflection cycle
// 2. Grace period expires → NEEDS_FEEDBACK
// 3. Publisher emits feedback request → REQUESTED
// 4. Inbound human feedback → RECEIVED + score overrides
func TestE2EFeedbackLoop(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	// 1. Create agent and run a reflection cycle
	g.UpsertNode("n:Agent:alice", "Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Message", graph.Properties{"text": "test message for reflection"})

	runner := reflectpkg.NewCycleRunner(g)
	now := time.Now()
	result, err := runner.Execute(context.Background(), reflectpkg.CycleRequest{
		Type:        reflectpkg.CycleDaily,
		AgentID:     "alice",
		WindowStart: now.Add(-1 * time.Hour),
		WindowEnd:   now.Add(1 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", result.Status)

	// Verify cycle starts with PENDING
	cycle, err := g.GetNode(result.CycleID)
	require.NoError(t, err)
	assert.Equal(t, "PENDING", cycle.Properties["humanFeedbackStatus"])

	// 2. Grace period check → NEEDS_FEEDBACK
	checker := reflectpkg.NewFeedbackChecker(g, 0) // immediate expiry
	err = checker.CheckPending(context.Background())
	require.NoError(t, err)

	cycle, err = g.GetNode(result.CycleID)
	require.NoError(t, err)
	assert.Equal(t, "NEEDS_FEEDBACK", cycle.Properties["humanFeedbackStatus"])

	// 3. Publisher check → REQUESTED + event emitted
	pub := ingest.NewMemoryPublisher()
	checker.WithPublisher(pub, "kafgraph.feedback.requests", 5)
	err = checker.CheckPending(context.Background())
	require.NoError(t, err)

	cycle, err = g.GetNode(result.CycleID)
	require.NoError(t, err)
	assert.Equal(t, "REQUESTED", cycle.Properties["humanFeedbackStatus"])
	assert.Equal(t, 1, pub.Len())

	// Verify published event
	var event reflectpkg.FeedbackRequestEvent
	require.NoError(t, json.Unmarshal(pub.Messages[0].Data, &event))
	assert.Equal(t, string(result.CycleID), event.CycleID)
	assert.Equal(t, "alice", event.AgentID)

	// 4. Inbound human feedback via handler → RECEIVED
	router := ingest.NewRouter()
	fbPayload, _ := json.Marshal(ingest.HumanFeedbackPayload{
		CycleID:      string(result.CycleID),
		FeedbackType: "positive",
		Comment:      "great insights from alice",
		Impact:       0.95,
		ReviewerID:   "reviewer-1",
	})
	env := &ingest.GroupEnvelope{
		Type:     ingest.TypeHumanFeedback,
		SenderID: "reviewer-1",
		Payload:  fbPayload,
	}
	src := ingest.SourceOffset{Topic: "feedback", Partition: 0, Offset: 1}
	err = router.Route(context.Background(), g, env, src)
	require.NoError(t, err)

	// Verify cycle is now RECEIVED
	cycle, err = g.GetNode(result.CycleID)
	require.NoError(t, err)
	assert.Equal(t, "RECEIVED", cycle.Properties["humanFeedbackStatus"])

	// Verify HumanFeedback node was created
	fbs, err := g.NodesByLabel("HumanFeedback")
	require.NoError(t, err)
	assert.Len(t, fbs, 1)
	assert.Equal(t, "positive", fbs[0].Properties["feedbackType"])
}

// TestE2EFeedbackWaive tests the waive flow.
func TestE2EFeedbackWaive(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	// Create a cycle in NEEDS_FEEDBACK state
	g.UpsertNode("n:ReflectionCycle:waive-test", "ReflectionCycle", graph.Properties{
		"status":              "COMPLETED",
		"humanFeedbackStatus": "NEEDS_FEEDBACK",
		"agentId":             "bob",
	})

	// Waive it
	g.UpsertNode("n:ReflectionCycle:waive-test", "ReflectionCycle", graph.Properties{
		"humanFeedbackStatus": "WAIVED",
	})

	cycle, err := g.GetNode("n:ReflectionCycle:waive-test")
	require.NoError(t, err)
	assert.Equal(t, "WAIVED", cycle.Properties["humanFeedbackStatus"])

	// Verify FeedbackChecker skips waived cycles
	pub := ingest.NewMemoryPublisher()
	checker := reflectpkg.NewFeedbackChecker(g, 0)
	checker.WithPublisher(pub, "test.topic", 5)
	err = checker.CheckPending(context.Background())
	require.NoError(t, err)

	// No messages should be published for waived cycles
	assert.Equal(t, 0, pub.Len())
}

// TestE2ESchedulerWithPublisher tests the scheduler wires publisher correctly.
func TestE2ESchedulerWithPublisher(t *testing.T) {
	store, err := storage.NewBadgerStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	g := graph.New(store)
	defer g.Close()

	pub := ingest.NewMemoryPublisher()
	sched := reflectpkg.NewScheduler(g, reflectpkg.SchedulerConfig{
		CheckInterval: 10 * time.Millisecond,
		GracePeriod:   0,
		Publisher:     pub,
		RequestTopic:  "test.feedback",
		TopN:          3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = sched.Run(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
