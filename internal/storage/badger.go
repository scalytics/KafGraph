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

// Package storage provides graph persistence backends.
package storage

import (
	"encoding/json"
	"fmt"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/index"
)

const (
	nodePrefix = "node:"
	edgePrefix = "edge:"
)

// BadgerStorage implements graph.Storage and graph.IndexedStorage using BadgerDB.
type BadgerStorage struct {
	db  *badger.DB
	idx *index.BadgerIndex
}

// NewBadgerStorage opens a BadgerDB at the given path.
func NewBadgerStorage(path string) (*BadgerStorage, error) {
	opts := badger.DefaultOptions(path).
		WithLoggingLevel(badger.WARNING)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	return &BadgerStorage{
		db:  db,
		idx: index.NewBadgerIndex(db),
	}, nil
}

// DB returns the underlying BadgerDB instance.
func (s *BadgerStorage) DB() *badger.DB {
	return s.db
}

func nodeKey(id graph.NodeID) []byte { return []byte(nodePrefix + string(id)) }
func edgeKey(id graph.EdgeID) []byte { return []byte(edgePrefix + string(id)) }

// PutNode creates or updates a node in the store with index maintenance.
func (s *BadgerStorage) PutNode(node *graph.Node) error {
	data, err := json.Marshal(node)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		// Deindex old version if it exists.
		if old, err := s.getNodeInTxn(txn, node.ID); err == nil {
			if err := s.idx.DeindexNode(txn, old); err != nil {
				return err
			}
		}
		if err := txn.Set(nodeKey(node.ID), data); err != nil {
			return err
		}
		return s.idx.IndexNode(txn, node)
	})
}

// GetNode retrieves a node by ID.
func (s *BadgerStorage) GetNode(id graph.NodeID) (*graph.Node, error) {
	var node graph.Node
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(nodeKey(id))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &node)
		})
	})
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("node %s not found", id)
		}
		return nil, err
	}
	return &node, nil
}

func (s *BadgerStorage) getNodeInTxn(txn *badger.Txn, id graph.NodeID) (*graph.Node, error) {
	item, err := txn.Get(nodeKey(id))
	if err != nil {
		return nil, err
	}
	var node graph.Node
	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, &node)
	})
	return &node, err
}

func (s *BadgerStorage) getEdgeInTxn(txn *badger.Txn, id graph.EdgeID) (*graph.Edge, error) {
	item, err := txn.Get(edgeKey(id))
	if err != nil {
		return nil, err
	}
	var edge graph.Edge
	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, &edge)
	})
	return &edge, err
}

// DeleteNode removes a node and its connected edges with index cleanup.
func (s *BadgerStorage) DeleteNode(id graph.NodeID) error {
	return s.db.Update(func(txn *badger.Txn) error {
		node, err := s.getNodeInTxn(txn, id)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}
			return err
		}
		// Find and remove connected edges via index.
		outIDs, _ := s.idx.OutgoingEdges(id)
		inIDs, _ := s.idx.IncomingEdges(id)
		allEdgeIDs := append(outIDs, inIDs...)
		seen := make(map[graph.EdgeID]bool)
		for _, eid := range allEdgeIDs {
			if seen[eid] {
				continue
			}
			seen[eid] = true
			if edge, err := s.getEdgeInTxn(txn, eid); err == nil {
				_ = s.idx.DeindexEdge(txn, edge)
				_ = txn.Delete(edgeKey(eid))
			}
		}
		if err := s.idx.DeindexNode(txn, node); err != nil {
			return err
		}
		return txn.Delete(nodeKey(id))
	})
}

// PutEdge creates or updates an edge in the store with index maintenance.
func (s *BadgerStorage) PutEdge(edge *graph.Edge) error {
	data, err := json.Marshal(edge)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		// Deindex old version if it exists.
		if old, err := s.getEdgeInTxn(txn, edge.ID); err == nil {
			if err := s.idx.DeindexEdge(txn, old); err != nil {
				return err
			}
		}
		if err := txn.Set(edgeKey(edge.ID), data); err != nil {
			return err
		}
		return s.idx.IndexEdge(txn, edge)
	})
}

// GetEdge retrieves an edge by ID.
func (s *BadgerStorage) GetEdge(id graph.EdgeID) (*graph.Edge, error) {
	var edge graph.Edge
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(edgeKey(id))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &edge)
		})
	})
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("edge %s not found", id)
		}
		return nil, err
	}
	return &edge, nil
}

// DeleteEdge removes an edge from the store with index cleanup.
func (s *BadgerStorage) DeleteEdge(id graph.EdgeID) error {
	return s.db.Update(func(txn *badger.Txn) error {
		if edge, err := s.getEdgeInTxn(txn, id); err == nil {
			if err := s.idx.DeindexEdge(txn, edge); err != nil {
				return err
			}
		}
		return txn.Delete(edgeKey(id))
	})
}

// NodesByLabel returns all nodes with the given label using the index.
func (s *BadgerStorage) NodesByLabel(label string) ([]*graph.Node, error) {
	ids, err := s.idx.NodesByLabel(label)
	if err != nil {
		return nil, err
	}
	var nodes []*graph.Node
	for _, id := range ids {
		node, err := s.GetNode(id)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// EdgesByNode returns all edges connected to a node using the index.
func (s *BadgerStorage) EdgesByNode(id graph.NodeID) ([]*graph.Edge, error) {
	outIDs, err := s.idx.OutgoingEdges(id)
	if err != nil {
		return nil, err
	}
	inIDs, err := s.idx.IncomingEdges(id)
	if err != nil {
		return nil, err
	}
	seen := make(map[graph.EdgeID]bool)
	var edges []*graph.Edge
	for _, eid := range append(outIDs, inIDs...) {
		if seen[eid] {
			continue
		}
		seen[eid] = true
		edge, err := s.GetEdge(eid)
		if err != nil {
			continue
		}
		edges = append(edges, edge)
	}
	return edges, nil
}

// NodeIDsByLabel returns node IDs with the given label (IndexedStorage).
func (s *BadgerStorage) NodeIDsByLabel(label string) ([]graph.NodeID, error) {
	return s.idx.NodesByLabel(label)
}

// OutgoingEdgeIDs returns edge IDs outgoing from the given node (IndexedStorage).
func (s *BadgerStorage) OutgoingEdgeIDs(nodeID graph.NodeID) ([]graph.EdgeID, error) {
	return s.idx.OutgoingEdges(nodeID)
}

// IncomingEdgeIDs returns edge IDs incoming to the given node (IndexedStorage).
func (s *BadgerStorage) IncomingEdgeIDs(nodeID graph.NodeID) ([]graph.EdgeID, error) {
	return s.idx.IncomingEdges(nodeID)
}

// EdgeIDsByLabel returns edge IDs with the given label (IndexedStorage).
func (s *BadgerStorage) EdgeIDsByLabel(label string) ([]graph.EdgeID, error) {
	return s.idx.EdgesByLabel(label)
}

// Close closes the BadgerDB database.
func (s *BadgerStorage) Close() error {
	return s.db.Close()
}
