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
	"fmt"
	"strconv"

	"github.com/scalytics/kafgraph/internal/graph"
)

// CheckpointStore persists last-processed offsets per topic-partition
// as _checkpoint-labeled nodes in the graph storage.
type CheckpointStore struct {
	g         *graph.Graph
	namespace string
}

// NewCheckpointStore creates a CheckpointStore.
func NewCheckpointStore(g *graph.Graph, namespace string) *CheckpointStore {
	return &CheckpointStore{g: g, namespace: namespace}
}

func (cs *CheckpointStore) nodeID(topic string, partition int32) graph.NodeID {
	return graph.NodeID(fmt.Sprintf("ckpt:%s:%s:%d", cs.namespace, topic, partition))
}

// Load returns the last-committed offset for a topic-partition, or -1 if none.
func (cs *CheckpointStore) Load(topic string, partition int32) (int64, error) {
	node, err := cs.g.GetNode(cs.nodeID(topic, partition))
	if err != nil {
		return -1, nil //nolint:nilerr // missing checkpoint is not an error
	}

	offsetStr, ok := node.Properties["offset"].(string)
	if !ok {
		return -1, nil
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("checkpoint load: %w", err)
	}
	return offset, nil
}

// Commit stores the offset for a topic-partition.
func (cs *CheckpointStore) Commit(topic string, partition int32, offset int64) error {
	_, err := cs.g.UpsertNode(cs.nodeID(topic, partition), "_checkpoint", graph.Properties{
		"topic":     topic,
		"partition": partition,
		"offset":    strconv.FormatInt(offset, 10),
		"namespace": cs.namespace,
	})
	return err
}
