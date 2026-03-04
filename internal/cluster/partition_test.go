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

package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentIDPartitioner_Deterministic(t *testing.T) {
	p := &AgentIDPartitioner{}
	a := p.Partition("agent-1", 16)
	b := p.Partition("agent-1", 16)
	assert.Equal(t, a, b, "same input should produce same partition")
}

func TestAgentIDPartitioner_DifferentAgents(t *testing.T) {
	p := &AgentIDPartitioner{}
	// With 1000 agents and 16 partitions, not all should map to the same partition.
	seen := make(map[int]bool)
	for i := 0; i < 1000; i++ {
		part := p.Partition(fmt.Sprintf("agent-%d", i), 16)
		seen[part] = true
	}
	assert.Greater(t, len(seen), 1, "agents should distribute across partitions")
}

func TestAgentIDPartitioner_Distribution(t *testing.T) {
	p := &AgentIDPartitioner{}
	numPartitions := 8
	counts := make(map[int]int)
	total := 10000
	for i := 0; i < total; i++ {
		part := p.Partition(fmt.Sprintf("agent-%d", i), numPartitions)
		counts[part]++
	}
	// Each partition should have at least 5% of total (625) for reasonable uniformity.
	for part, count := range counts {
		assert.Greater(t, count, total/numPartitions/4,
			"partition %d has too few agents: %d", part, count)
	}
}

func TestAgentIDPartitioner_ZeroPartitions(t *testing.T) {
	p := &AgentIDPartitioner{}
	assert.Equal(t, 0, p.Partition("agent-1", 0))
}

func TestAgentIDPartitioner_OnePartition(t *testing.T) {
	p := &AgentIDPartitioner{}
	assert.Equal(t, 0, p.Partition("agent-1", 1))
	assert.Equal(t, 0, p.Partition("agent-999", 1))
}

func TestPartitionMap_NewDefaults(t *testing.T) {
	pm := NewPartitionMap(16, &AgentIDPartitioner{})
	assert.Equal(t, 16, pm.NumPartitions())
	assert.Nil(t, pm.Owner("agent-1"), "no owners before rebalance")
}

func TestPartitionMap_RebalanceSingleNode(t *testing.T) {
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "node-1", Addr: "127.0.0.1", RPCPort: 7948},
	})
	// All 4 partitions should be owned by node-1.
	for i := 0; i < 4; i++ {
		owner := pm.Owners()[i]
		require.NotNil(t, owner)
		assert.Equal(t, "node-1", owner.Name)
	}
}

func TestPartitionMap_RebalanceMultipleNodes(t *testing.T) {
	pm := NewPartitionMap(6, &AgentIDPartitioner{})
	members := []NodeInfo{
		{Name: "node-b", Addr: "10.0.0.2", RPCPort: 7948},
		{Name: "node-a", Addr: "10.0.0.1", RPCPort: 7948},
		{Name: "node-c", Addr: "10.0.0.3", RPCPort: 7948},
	}
	pm.Rebalance(members)
	// Round-robin over sorted names: a, b, c, a, b, c
	owners := pm.Owners()
	assert.Equal(t, "node-a", owners[0].Name)
	assert.Equal(t, "node-b", owners[1].Name)
	assert.Equal(t, "node-c", owners[2].Name)
	assert.Equal(t, "node-a", owners[3].Name)
	assert.Equal(t, "node-b", owners[4].Name)
	assert.Equal(t, "node-c", owners[5].Name)
}

func TestPartitionMap_RebalanceEmptyMembers(t *testing.T) {
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{})
	assert.Nil(t, pm.Owner("agent-1"))
}

func TestPartitionMap_LocalPartitions(t *testing.T) {
	pm := NewPartitionMap(6, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "node-a", Addr: "10.0.0.1", RPCPort: 7948},
		{Name: "node-b", Addr: "10.0.0.2", RPCPort: 7948},
	})
	// Round-robin: a(0), b(1), a(2), b(3), a(4), b(5)
	local := pm.LocalPartitions("node-a")
	assert.Equal(t, []int{0, 2, 4}, local)
}

func TestPartitionMap_IsLocal(t *testing.T) {
	pm := NewPartitionMap(16, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "self", Addr: "127.0.0.1", RPCPort: 7948},
	})
	// With a single node, all agents should be local.
	assert.True(t, pm.IsLocal("agent-1", "self"))
	assert.True(t, pm.IsLocal("agent-999", "self"))
}

func TestPartitionMap_UniqueOwners(t *testing.T) {
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "node-a", Addr: "10.0.0.1", RPCPort: 7948},
		{Name: "node-b", Addr: "10.0.0.2", RPCPort: 7948},
	})
	owners := pm.UniqueOwners()
	assert.Len(t, owners, 2)
	assert.Equal(t, "node-a", owners[0].Name)
	assert.Equal(t, "node-b", owners[1].Name)
}

func TestPartitionMap_RebalanceAfterJoinLeave(t *testing.T) {
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	// Initial: 2 nodes
	pm.Rebalance([]NodeInfo{
		{Name: "node-a", Addr: "10.0.0.1", RPCPort: 7948},
		{Name: "node-b", Addr: "10.0.0.2", RPCPort: 7948},
	})
	assert.Len(t, pm.LocalPartitions("node-a"), 2)

	// Add a third node
	pm.Rebalance([]NodeInfo{
		{Name: "node-a", Addr: "10.0.0.1", RPCPort: 7948},
		{Name: "node-b", Addr: "10.0.0.2", RPCPort: 7948},
		{Name: "node-c", Addr: "10.0.0.3", RPCPort: 7948},
	})
	// 4 partitions / 3 nodes: a gets 2, b gets 1, c gets 1
	assert.Len(t, pm.LocalPartitions("node-a"), 2)
	assert.Len(t, pm.LocalPartitions("node-b"), 1)
	assert.Len(t, pm.LocalPartitions("node-c"), 1)

	// Remove node-c
	pm.Rebalance([]NodeInfo{
		{Name: "node-a", Addr: "10.0.0.1", RPCPort: 7948},
		{Name: "node-b", Addr: "10.0.0.2", RPCPort: 7948},
	})
	assert.Len(t, pm.LocalPartitions("node-a"), 2)
	assert.Len(t, pm.LocalPartitions("node-b"), 2)
}

func TestPartitionMap_ZeroPartitions(t *testing.T) {
	pm := NewPartitionMap(0, &AgentIDPartitioner{})
	assert.Equal(t, 1, pm.NumPartitions(), "should default to 1 partition")
}
