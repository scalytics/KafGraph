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

package index

import (
	"strings"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/scalytics/kafgraph/internal/graph"
)

// Key prefixes for secondary indexes.
const (
	prefixLabel     = "i:lbl:"  // i:lbl:<label>:<nodeID>
	prefixOutgoing  = "i:out:"  // i:out:<nodeID>:<edgeID>
	prefixIncoming  = "i:in:"   // i:in:<nodeID>:<edgeID>
	prefixEdgeLabel = "i:elbl:" // i:elbl:<label>:<edgeID>
)

// Manager defines the interface for maintaining secondary indexes.
type Manager interface {
	IndexNode(txn *badger.Txn, node *graph.Node) error
	DeindexNode(txn *badger.Txn, node *graph.Node) error
	IndexEdge(txn *badger.Txn, edge *graph.Edge) error
	DeindexEdge(txn *badger.Txn, edge *graph.Edge) error
	NodesByLabel(label string) ([]graph.NodeID, error)
	OutgoingEdges(nodeID graph.NodeID) ([]graph.EdgeID, error)
	IncomingEdges(nodeID graph.NodeID) ([]graph.EdgeID, error)
	EdgesByLabel(label string) ([]graph.EdgeID, error)
}

// BadgerIndex implements Manager using BadgerDB prefix scans.
type BadgerIndex struct {
	db *badger.DB
}

// NewBadgerIndex creates a new index manager backed by the given BadgerDB.
func NewBadgerIndex(db *badger.DB) *BadgerIndex {
	return &BadgerIndex{db: db}
}

// IndexNode adds secondary index entries for a node.
func (idx *BadgerIndex) IndexNode(txn *badger.Txn, node *graph.Node) error {
	key := prefixLabel + node.Label + ":" + string(node.ID)
	return txn.Set([]byte(key), nil)
}

// DeindexNode removes secondary index entries for a node.
func (idx *BadgerIndex) DeindexNode(txn *badger.Txn, node *graph.Node) error {
	key := prefixLabel + node.Label + ":" + string(node.ID)
	return txn.Delete([]byte(key))
}

// IndexEdge adds secondary index entries for an edge.
func (idx *BadgerIndex) IndexEdge(txn *badger.Txn, edge *graph.Edge) error {
	outKey := prefixOutgoing + string(edge.FromID) + ":" + string(edge.ID)
	if err := txn.Set([]byte(outKey), nil); err != nil {
		return err
	}
	inKey := prefixIncoming + string(edge.ToID) + ":" + string(edge.ID)
	if err := txn.Set([]byte(inKey), nil); err != nil {
		return err
	}
	lblKey := prefixEdgeLabel + edge.Label + ":" + string(edge.ID)
	return txn.Set([]byte(lblKey), nil)
}

// DeindexEdge removes secondary index entries for an edge.
func (idx *BadgerIndex) DeindexEdge(txn *badger.Txn, edge *graph.Edge) error {
	outKey := prefixOutgoing + string(edge.FromID) + ":" + string(edge.ID)
	if err := txn.Delete([]byte(outKey)); err != nil {
		return err
	}
	inKey := prefixIncoming + string(edge.ToID) + ":" + string(edge.ID)
	if err := txn.Delete([]byte(inKey)); err != nil {
		return err
	}
	lblKey := prefixEdgeLabel + edge.Label + ":" + string(edge.ID)
	return txn.Delete([]byte(lblKey))
}

// NodesByLabel returns all node IDs with the given label using the index.
func (idx *BadgerIndex) NodesByLabel(label string) ([]graph.NodeID, error) {
	prefix := prefixLabel + label + ":"
	var ids []graph.NodeID
	err := idx.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			id := strings.TrimPrefix(key, prefix)
			ids = append(ids, graph.NodeID(id))
		}
		return nil
	})
	return ids, err
}

// OutgoingEdges returns all edge IDs outgoing from the given node.
func (idx *BadgerIndex) OutgoingEdges(nodeID graph.NodeID) ([]graph.EdgeID, error) {
	prefix := prefixOutgoing + string(nodeID) + ":"
	var ids []graph.EdgeID
	err := idx.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			id := strings.TrimPrefix(key, prefix)
			ids = append(ids, graph.EdgeID(id))
		}
		return nil
	})
	return ids, err
}

// IncomingEdges returns all edge IDs incoming to the given node.
func (idx *BadgerIndex) IncomingEdges(nodeID graph.NodeID) ([]graph.EdgeID, error) {
	prefix := prefixIncoming + string(nodeID) + ":"
	var ids []graph.EdgeID
	err := idx.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			id := strings.TrimPrefix(key, prefix)
			ids = append(ids, graph.EdgeID(id))
		}
		return nil
	})
	return ids, err
}

// EdgesByLabel returns all edge IDs with the given label.
func (idx *BadgerIndex) EdgesByLabel(label string) ([]graph.EdgeID, error) {
	prefix := prefixEdgeLabel + label + ":"
	var ids []graph.EdgeID
	err := idx.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			id := strings.TrimPrefix(key, prefix)
			ids = append(ids, graph.EdgeID(id))
		}
		return nil
	})
	return ids, err
}
