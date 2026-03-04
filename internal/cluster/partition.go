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
	"hash/fnv"
	"maps"
	"sort"
	"sync"
)

// AgentIDPartitioner uses FNV-1a hashing for deterministic partition assignment.
type AgentIDPartitioner struct{}

// Partition returns the partition number for the given agent ID.
func (p *AgentIDPartitioner) Partition(agentID string, numPartitions int) int {
	if numPartitions <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(agentID))
	return int(h.Sum32()) % numPartitions
}

// PartitionMap tracks which cluster node owns which partitions.
type PartitionMap struct {
	mu            sync.RWMutex
	numPartitions int
	strategy      PartitionStrategy
	owners        map[int]*NodeInfo // partition → owner
}

// NewPartitionMap creates a partition map with the given number of partitions
// and partitioning strategy.
func NewPartitionMap(numPartitions int, strategy PartitionStrategy) *PartitionMap {
	if numPartitions <= 0 {
		numPartitions = 1
	}
	return &PartitionMap{
		numPartitions: numPartitions,
		strategy:      strategy,
		owners:        make(map[int]*NodeInfo),
	}
}

// NumPartitions returns the total number of partitions.
func (pm *PartitionMap) NumPartitions() int {
	return pm.numPartitions
}

// Rebalance assigns partitions to members using deterministic round-robin.
// Members are sorted by name so all nodes compute the same map independently.
func (pm *PartitionMap) Rebalance(members []NodeInfo) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.owners = make(map[int]*NodeInfo)
	if len(members) == 0 {
		return
	}

	// Sort by name for deterministic assignment.
	sorted := make([]NodeInfo, len(members))
	copy(sorted, members)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	for i := 0; i < pm.numPartitions; i++ {
		owner := sorted[i%len(sorted)]
		pm.owners[i] = &NodeInfo{
			Name:     owner.Name,
			Addr:     owner.Addr,
			RPCPort:  owner.RPCPort,
			BoltPort: owner.BoltPort,
			HTTPPort: owner.HTTPPort,
		}
	}
}

// Owner returns the node that owns the partition for the given agent ID.
// Returns nil if no members have been assigned.
func (pm *PartitionMap) Owner(agentID string) *NodeInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	partition := pm.strategy.Partition(agentID, pm.numPartitions)
	return pm.owners[partition]
}

// LocalPartitions returns the partition numbers owned by the named node.
func (pm *PartitionMap) LocalPartitions(self string) []int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var partitions []int
	for p, owner := range pm.owners {
		if owner.Name == self {
			partitions = append(partitions, p)
		}
	}
	sort.Ints(partitions)
	return partitions
}

// IsLocal returns true if the partition for agentID is owned by self.
func (pm *PartitionMap) IsLocal(agentID string, self string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	partition := pm.strategy.Partition(agentID, pm.numPartitions)
	owner := pm.owners[partition]
	return owner != nil && owner.Name == self
}

// Owners returns a snapshot of the current partition→owner mapping.
func (pm *PartitionMap) Owners() map[int]*NodeInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[int]*NodeInfo, len(pm.owners))
	maps.Copy(result, pm.owners)
	return result
}

// UniqueOwners returns the deduplicated list of nodes that own at least one partition.
func (pm *PartitionMap) UniqueOwners() []NodeInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	seen := make(map[string]NodeInfo)
	for _, owner := range pm.owners {
		if owner != nil {
			seen[owner.Name] = *owner
		}
	}

	result := make([]NodeInfo, 0, len(seen))
	for _, n := range seen {
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}
