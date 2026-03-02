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
	"crypto/sha256"
	"fmt"

	"github.com/scalytics/kafgraph/internal/graph"
)

// AgentNodeID returns a deterministic node ID for an agent.
func AgentNodeID(agentID string) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("n:Agent:%s", agentID))
}

// ConversationNodeID returns a deterministic node ID for a conversation.
func ConversationNodeID(correlationID string) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("n:Conversation:%s", correlationID))
}

// MessageNodeID returns a deterministic node ID for a message.
func MessageNodeID(src SourceOffset) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("n:Message:%s:%d:%d", src.Topic, src.Partition, src.Offset))
}

// SkillNodeID returns a deterministic node ID for a skill.
func SkillNodeID(skillName string) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("n:Skill:%s", skillName))
}

// SharedMemoryNodeID returns a deterministic node ID for a shared memory entry.
func SharedMemoryNodeID(src SourceOffset) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("n:SharedMemory:%s:%d:%d", src.Topic, src.Partition, src.Offset))
}

// AuditEventNodeID returns a deterministic node ID for an audit event.
func AuditEventNodeID(src SourceOffset) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("n:AuditEvent:%s:%d:%d", src.Topic, src.Partition, src.Offset))
}

// DeterministicEdgeID returns a deterministic edge ID from label and endpoints.
func DeterministicEdgeID(label string, from, to graph.NodeID) graph.EdgeID {
	h := sha256.Sum256(fmt.Appendf(nil, "%s:%s", from, to))
	return graph.EdgeID(fmt.Sprintf("e:%s:%x", label, h[:8]))
}
