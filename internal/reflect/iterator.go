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
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// HistoricIterator provides time-windowed graph node traversal.
type HistoricIterator struct {
	graph *graph.Graph
}

// NewHistoricIterator creates a new iterator.
func NewHistoricIterator(g *graph.Graph) *HistoricIterator {
	return &HistoricIterator{graph: g}
}

// NodesInWindow returns nodes of given labels with CreatedAt in [start, end).
func (it *HistoricIterator) NodesInWindow(labels []string, start, end time.Time) ([]*graph.Node, error) {
	var result []*graph.Node
	for _, label := range labels {
		nodes, err := it.graph.NodesByLabel(label)
		if err != nil {
			continue
		}
		for _, n := range nodes {
			if !n.CreatedAt.Before(start) && n.CreatedAt.Before(end) {
				result = append(result, n)
			}
		}
	}
	if result == nil {
		result = []*graph.Node{}
	}
	return result, nil
}

// AgentNodesInWindow returns nodes connected to agentID within [start, end),
// filtered by the given labels.
func (it *HistoricIterator) AgentNodesInWindow(agentID graph.NodeID, labels []string, start, end time.Time) ([]*graph.Node, error) {
	labelSet := make(map[string]bool, len(labels))
	for _, l := range labels {
		labelSet[l] = true
	}

	edges, err := it.graph.Neighbors(agentID)
	if err != nil {
		return []*graph.Node{}, nil
	}

	seen := make(map[graph.NodeID]bool)
	var result []*graph.Node
	for _, edge := range edges {
		targetID := edge.ToID
		if targetID == agentID {
			targetID = edge.FromID
		}
		if seen[targetID] {
			continue
		}
		seen[targetID] = true

		node, err := it.graph.GetNode(targetID)
		if err != nil {
			continue
		}
		if !labelSet[node.Label] {
			continue
		}
		if node.CreatedAt.Before(start) || !node.CreatedAt.Before(end) {
			continue
		}
		result = append(result, node)
	}
	if result == nil {
		result = []*graph.Node{}
	}
	return result, nil
}
