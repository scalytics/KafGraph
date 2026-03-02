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

package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentNodeID(t *testing.T) {
	id := AgentNodeID("alice")
	assert.Equal(t, "n:Agent:alice", string(id))
}

func TestConversationNodeID(t *testing.T) {
	id := ConversationNodeID("corr-123")
	assert.Equal(t, "n:Conversation:corr-123", string(id))
}

func TestMessageNodeID(t *testing.T) {
	src := SourceOffset{Topic: "group.chat", Partition: 0, Offset: 42}
	id := MessageNodeID(src)
	assert.Equal(t, "n:Message:group.chat:0:42", string(id))
}

func TestSkillNodeID(t *testing.T) {
	id := SkillNodeID("brain_search")
	assert.Equal(t, "n:Skill:brain_search", string(id))
}

func TestSharedMemoryNodeID(t *testing.T) {
	src := SourceOffset{Topic: "group.memory", Partition: 1, Offset: 7}
	id := SharedMemoryNodeID(src)
	assert.Equal(t, "n:SharedMemory:group.memory:1:7", string(id))
}

func TestAuditEventNodeID(t *testing.T) {
	src := SourceOffset{Topic: "group.audit", Partition: 0, Offset: 99}
	id := AuditEventNodeID(src)
	assert.Equal(t, "n:AuditEvent:group.audit:0:99", string(id))
}

func TestDeterministicEdgeIDStable(t *testing.T) {
	from := AgentNodeID("alice")
	to := ConversationNodeID("corr-1")

	id1 := DeterministicEdgeID("AUTHORED", from, to)
	id2 := DeterministicEdgeID("AUTHORED", from, to)
	assert.Equal(t, id1, id2, "same inputs must produce same ID")
}

func TestDeterministicEdgeIDUnique(t *testing.T) {
	from := AgentNodeID("alice")
	to1 := ConversationNodeID("corr-1")
	to2 := ConversationNodeID("corr-2")

	id1 := DeterministicEdgeID("AUTHORED", from, to1)
	id2 := DeterministicEdgeID("AUTHORED", from, to2)
	assert.NotEqual(t, id1, id2, "different endpoints must produce different IDs")
}

func TestDeterministicEdgeIDDifferentLabels(t *testing.T) {
	from := AgentNodeID("alice")
	to := ConversationNodeID("corr-1")

	id1 := DeterministicEdgeID("AUTHORED", from, to)
	id2 := DeterministicEdgeID("BELONGS_TO", from, to)
	assert.NotEqual(t, id1, id2, "different labels must produce different IDs")
}
