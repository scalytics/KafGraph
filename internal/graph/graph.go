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

// Package graph provides the core property graph data model and CRUD operations.
package graph

import (
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"
)

var idCounter atomic.Int64

// NodeID uniquely identifies a node in the graph.
type NodeID string

// EdgeID uniquely identifies an edge in the graph.
type EdgeID string

// Properties is a map of key-value properties on nodes and edges.
type Properties map[string]any

// Node represents a labeled property graph node.
type Node struct {
	ID         NodeID     `json:"id"`
	Label      string     `json:"label"`
	Properties Properties `json:"properties"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// Edge represents a directed labeled property graph edge.
type Edge struct {
	ID         EdgeID     `json:"id"`
	Label      string     `json:"label"`
	FromID     NodeID     `json:"fromId"`
	ToID       NodeID     `json:"toId"`
	Properties Properties `json:"properties"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// Storage defines the interface for graph persistence backends.
type Storage interface {
	// PutNode creates or updates a node.
	PutNode(node *Node) error
	// GetNode retrieves a node by ID.
	GetNode(id NodeID) (*Node, error)
	// DeleteNode removes a node and its connected edges.
	DeleteNode(id NodeID) error

	// PutEdge creates or updates an edge.
	PutEdge(edge *Edge) error
	// GetEdge retrieves an edge by ID.
	GetEdge(id EdgeID) (*Edge, error)
	// DeleteEdge removes an edge.
	DeleteEdge(id EdgeID) error

	// NodesByLabel returns all nodes with the given label.
	NodesByLabel(label string) ([]*Node, error)
	// EdgesByNode returns all edges connected to a node.
	EdgesByNode(id NodeID) ([]*Edge, error)

	// Close releases storage resources.
	Close() error
}

// Graph is the core graph engine providing CRUD operations over a Storage backend.
type Graph struct {
	mu      sync.RWMutex
	storage Storage
}

// New creates a new Graph backed by the given Storage implementation.
func New(storage Storage) *Graph {
	return &Graph{storage: storage}
}

// CreateNode adds a new node to the graph.
func (g *Graph) CreateNode(label string, props Properties) (*Node, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node := &Node{
		ID:         NodeID(fmt.Sprintf("n:%s:%d", label, idCounter.Add(1))),
		Label:      label,
		Properties: props,
		CreatedAt:  time.Now().UTC(),
	}

	if err := g.storage.PutNode(node); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	return node, nil
}

// GetNode retrieves a node by its ID.
func (g *Graph) GetNode(id NodeID) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.storage.GetNode(id)
}

// DeleteNode removes a node from the graph.
func (g *Graph) DeleteNode(id NodeID) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.storage.DeleteNode(id)
}

// CreateEdge adds a directed edge between two nodes.
func (g *Graph) CreateEdge(label string, from, to NodeID, props Properties) (*Edge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Verify both endpoints exist
	if _, err := g.storage.GetNode(from); err != nil {
		return nil, fmt.Errorf("source node %s: %w", from, err)
	}
	if _, err := g.storage.GetNode(to); err != nil {
		return nil, fmt.Errorf("target node %s: %w", to, err)
	}

	edge := &Edge{
		ID:         EdgeID(fmt.Sprintf("e:%s:%d", label, idCounter.Add(1))),
		Label:      label,
		FromID:     from,
		ToID:       to,
		Properties: props,
		CreatedAt:  time.Now().UTC(),
	}

	if err := g.storage.PutEdge(edge); err != nil {
		return nil, fmt.Errorf("create edge: %w", err)
	}
	return edge, nil
}

// GetEdge retrieves an edge by its ID.
func (g *Graph) GetEdge(id EdgeID) (*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.storage.GetEdge(id)
}

// DeleteEdge removes an edge from the graph.
func (g *Graph) DeleteEdge(id EdgeID) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.storage.DeleteEdge(id)
}

// NodesByLabel returns all nodes with the given label.
func (g *Graph) NodesByLabel(label string) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.storage.NodesByLabel(label)
}

// Neighbors returns all edges connected to the given node.
func (g *Graph) Neighbors(id NodeID) ([]*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.storage.EdgesByNode(id)
}

// UpsertNode creates a node with the given ID or merges properties if it exists.
func (g *Graph) UpsertNode(id NodeID, label string, props Properties) (*Node, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	existing, err := g.storage.GetNode(id)
	if err == nil {
		// Merge properties: new keys overwrite existing.
		maps.Copy(existing.Properties, props)
		if err := g.storage.PutNode(existing); err != nil {
			return nil, fmt.Errorf("upsert node: %w", err)
		}
		return existing, nil
	}

	node := &Node{
		ID:         id,
		Label:      label,
		Properties: props,
		CreatedAt:  time.Now().UTC(),
	}
	if node.Properties == nil {
		node.Properties = Properties{}
	}
	if err := g.storage.PutNode(node); err != nil {
		return nil, fmt.Errorf("upsert node: %w", err)
	}
	return node, nil
}

// UpsertEdge creates an edge with the given ID or merges properties if it exists.
// Unlike CreateEdge, it does NOT verify that endpoints exist — during ingestion,
// out-of-order messages mean we might see a response before the agent's announce.
func (g *Graph) UpsertEdge(id EdgeID, label string, from, to NodeID, props Properties) (*Edge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	existing, err := g.storage.GetEdge(id)
	if err == nil {
		maps.Copy(existing.Properties, props)
		if err := g.storage.PutEdge(existing); err != nil {
			return nil, fmt.Errorf("upsert edge: %w", err)
		}
		return existing, nil
	}

	edge := &Edge{
		ID:         id,
		Label:      label,
		FromID:     from,
		ToID:       to,
		Properties: props,
		CreatedAt:  time.Now().UTC(),
	}
	if edge.Properties == nil {
		edge.Properties = Properties{}
	}
	if err := g.storage.PutEdge(edge); err != nil {
		return nil, fmt.Errorf("upsert edge: %w", err)
	}
	return edge, nil
}

// Close shuts down the graph and releases storage resources.
func (g *Graph) Close() error {
	return g.storage.Close()
}
