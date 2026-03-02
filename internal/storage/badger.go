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
)

const (
	nodePrefix = "node:"
	edgePrefix = "edge:"
)

// BadgerStorage implements graph.Storage using BadgerDB.
type BadgerStorage struct {
	db *badger.DB
}

// NewBadgerStorage opens a BadgerDB at the given path.
func NewBadgerStorage(path string) (*BadgerStorage, error) {
	opts := badger.DefaultOptions(path).
		WithLoggingLevel(badger.WARNING)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	return &BadgerStorage{db: db}, nil
}

func nodeKey(id graph.NodeID) []byte { return []byte(nodePrefix + string(id)) }
func edgeKey(id graph.EdgeID) []byte { return []byte(edgePrefix + string(id)) }

// PutNode creates or updates a node in the store.
func (s *BadgerStorage) PutNode(node *graph.Node) error {
	data, err := json.Marshal(node)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(nodeKey(node.ID), data)
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

// DeleteNode removes a node and its connected edges.
func (s *BadgerStorage) DeleteNode(id graph.NodeID) error {
	// Delete connected edges first
	edges, err := s.EdgesByNode(id)
	if err != nil {
		return err
	}
	for _, e := range edges {
		if err := s.DeleteEdge(e.ID); err != nil {
			return err
		}
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(nodeKey(id))
	})
}

// PutEdge creates or updates an edge in the store.
func (s *BadgerStorage) PutEdge(edge *graph.Edge) error {
	data, err := json.Marshal(edge)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(edgeKey(edge.ID), data)
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

// DeleteEdge removes an edge from the store.
func (s *BadgerStorage) DeleteEdge(id graph.EdgeID) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(edgeKey(id))
	})
}

// NodesByLabel returns all nodes with the given label.
func (s *BadgerStorage) NodesByLabel(label string) ([]*graph.Node, error) {
	var nodes []*graph.Node
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(nodePrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var node graph.Node
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &node)
			}); err != nil {
				return err
			}
			if node.Label == label {
				nodes = append(nodes, &node)
			}
		}
		return nil
	})
	return nodes, err
}

// EdgesByNode returns all edges connected to a node.
func (s *BadgerStorage) EdgesByNode(id graph.NodeID) ([]*graph.Edge, error) {
	var edges []*graph.Edge
	idStr := string(id)
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(edgePrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var edge graph.Edge
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &edge)
			}); err != nil {
				return err
			}
			if string(edge.FromID) == idStr || string(edge.ToID) == idStr {
				edges = append(edges, &edge)
			}
		}
		return nil
	})
	return edges, err
}

// Close closes the BadgerDB database.
func (s *BadgerStorage) Close() error {
	return s.db.Close()
}
