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
	"testing"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func newTestDB(t *testing.T) *badger.DB {
	t.Helper()
	opts := badger.DefaultOptions("").WithInMemory(true).WithLoggingLevel(badger.WARNING)
	db, err := badger.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestIndexNodeAndNodesByLabel(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	node := &graph.Node{ID: "n:Agent:alice", Label: "Agent", CreatedAt: time.Now()}
	err := db.Update(func(txn *badger.Txn) error {
		return idx.IndexNode(txn, node)
	})
	require.NoError(t, err)

	ids, err := idx.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Equal(t, []graph.NodeID{"n:Agent:alice"}, ids)
}

func TestNodesByLabelMultiple(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	err := db.Update(func(txn *badger.Txn) error {
		if err := idx.IndexNode(txn, &graph.Node{ID: "n:Agent:alice", Label: "Agent"}); err != nil {
			return err
		}
		return idx.IndexNode(txn, &graph.Node{ID: "n:Agent:bob", Label: "Agent"})
	})
	require.NoError(t, err)

	ids, err := idx.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Len(t, ids, 2)
}

func TestNodesByLabelEmpty(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	ids, err := idx.NodesByLabel("Nonexistent")
	require.NoError(t, err)
	assert.Nil(t, ids)
}

func TestDeindexNode(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	node := &graph.Node{ID: "n:Agent:alice", Label: "Agent"}
	err := db.Update(func(txn *badger.Txn) error {
		return idx.IndexNode(txn, node)
	})
	require.NoError(t, err)

	err = db.Update(func(txn *badger.Txn) error {
		return idx.DeindexNode(txn, node)
	})
	require.NoError(t, err)

	ids, err := idx.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Nil(t, ids)
}

func TestIndexEdgeAndOutgoing(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	edge := &graph.Edge{ID: "e:KNOWS:1", Label: "KNOWS", FromID: "n:a", ToID: "n:b"}
	err := db.Update(func(txn *badger.Txn) error {
		return idx.IndexEdge(txn, edge)
	})
	require.NoError(t, err)

	ids, err := idx.OutgoingEdges("n:a")
	require.NoError(t, err)
	assert.Equal(t, []graph.EdgeID{"e:KNOWS:1"}, ids)
}

func TestIndexEdgeAndIncoming(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	edge := &graph.Edge{ID: "e:KNOWS:1", Label: "KNOWS", FromID: "n:a", ToID: "n:b"}
	err := db.Update(func(txn *badger.Txn) error {
		return idx.IndexEdge(txn, edge)
	})
	require.NoError(t, err)

	ids, err := idx.IncomingEdges("n:b")
	require.NoError(t, err)
	assert.Equal(t, []graph.EdgeID{"e:KNOWS:1"}, ids)
}

func TestEdgesByLabel(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	err := db.Update(func(txn *badger.Txn) error {
		if err := idx.IndexEdge(txn, &graph.Edge{ID: "e:KNOWS:1", Label: "KNOWS", FromID: "n:a", ToID: "n:b"}); err != nil {
			return err
		}
		return idx.IndexEdge(txn, &graph.Edge{ID: "e:KNOWS:2", Label: "KNOWS", FromID: "n:b", ToID: "n:c"})
	})
	require.NoError(t, err)

	ids, err := idx.EdgesByLabel("KNOWS")
	require.NoError(t, err)
	assert.Len(t, ids, 2)
}

func TestEdgesByLabelEmpty(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	ids, err := idx.EdgesByLabel("UNKNOWN")
	require.NoError(t, err)
	assert.Nil(t, ids)
}

func TestDeindexEdge(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	edge := &graph.Edge{ID: "e:KNOWS:1", Label: "KNOWS", FromID: "n:a", ToID: "n:b"}
	err := db.Update(func(txn *badger.Txn) error {
		return idx.IndexEdge(txn, edge)
	})
	require.NoError(t, err)

	err = db.Update(func(txn *badger.Txn) error {
		return idx.DeindexEdge(txn, edge)
	})
	require.NoError(t, err)

	out, err := idx.OutgoingEdges("n:a")
	require.NoError(t, err)
	assert.Nil(t, out)

	in, err := idx.IncomingEdges("n:b")
	require.NoError(t, err)
	assert.Nil(t, in)

	lbl, err := idx.EdgesByLabel("KNOWS")
	require.NoError(t, err)
	assert.Nil(t, lbl)
}

func TestOutgoingEdgesMultiple(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	err := db.Update(func(txn *badger.Txn) error {
		if err := idx.IndexEdge(txn, &graph.Edge{ID: "e:KNOWS:1", Label: "KNOWS", FromID: "n:a", ToID: "n:b"}); err != nil {
			return err
		}
		return idx.IndexEdge(txn, &graph.Edge{ID: "e:LIKES:2", Label: "LIKES", FromID: "n:a", ToID: "n:c"})
	})
	require.NoError(t, err)

	ids, err := idx.OutgoingEdges("n:a")
	require.NoError(t, err)
	assert.Len(t, ids, 2)
}

func TestIncomingEdgesEmpty(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	ids, err := idx.IncomingEdges("n:nonexistent")
	require.NoError(t, err)
	assert.Nil(t, ids)
}

func TestNodesByLabelIsolation(t *testing.T) {
	db := newTestDB(t)
	idx := NewBadgerIndex(db)

	err := db.Update(func(txn *badger.Txn) error {
		if err := idx.IndexNode(txn, &graph.Node{ID: "n:Agent:alice", Label: "Agent"}); err != nil {
			return err
		}
		return idx.IndexNode(txn, &graph.Node{ID: "n:Message:1", Label: "Message"})
	})
	require.NoError(t, err)

	agents, err := idx.NodesByLabel("Agent")
	require.NoError(t, err)
	assert.Len(t, agents, 1)
	assert.Equal(t, graph.NodeID("n:Agent:alice"), agents[0])

	messages, err := idx.NodesByLabel("Message")
	require.NoError(t, err)
	assert.Len(t, messages, 1)
}
