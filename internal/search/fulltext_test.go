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

package search

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func newTestSearcher(t *testing.T) *BleveSearcher {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bleve.idx")
	fields := []LabelProperty{
		{Label: "Message", Property: "text"},
		{Label: "SharedMemory", Property: "value"},
	}
	bs, err := NewBleveSearcher(path, fields)
	require.NoError(t, err)
	t.Cleanup(func() { bs.Close() })
	return bs
}

func TestBleveIndexAndSearch(t *testing.T) {
	bs := newTestSearcher(t)

	node := &graph.Node{
		ID:         "n:Message:1",
		Label:      "Message",
		Properties: graph.Properties{"text": "hello world from alice"},
		CreatedAt:  time.Now(),
	}
	require.NoError(t, bs.Index(node))

	results, err := bs.Search("Message", "text", "hello", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, graph.NodeID("n:Message:1"), results[0].NodeID)
	assert.Greater(t, results[0].Score, 0.0)
}

func TestBleveSearchNoResults(t *testing.T) {
	bs := newTestSearcher(t)

	results, err := bs.Search("Message", "text", "nonexistent", 10)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestBleveSearchMultipleHits(t *testing.T) {
	bs := newTestSearcher(t)

	bs.Index(&graph.Node{
		ID: "n:Message:1", Label: "Message",
		Properties: graph.Properties{"text": "kubernetes deployment ready"},
		CreatedAt:  time.Now(),
	})
	bs.Index(&graph.Node{
		ID: "n:Message:2", Label: "Message",
		Properties: graph.Properties{"text": "kubernetes scaling up"},
		CreatedAt:  time.Now(),
	})

	results, err := bs.Search("Message", "text", "kubernetes", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestBleveRemove(t *testing.T) {
	bs := newTestSearcher(t)

	node := &graph.Node{
		ID: "n:Message:1", Label: "Message",
		Properties: graph.Properties{"text": "delete me please"},
		CreatedAt:  time.Now(),
	}
	require.NoError(t, bs.Index(node))
	require.NoError(t, bs.Remove("n:Message:1"))

	results, err := bs.Search("Message", "text", "delete", 10)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestBleveLabelIsolation(t *testing.T) {
	bs := newTestSearcher(t)

	bs.Index(&graph.Node{
		ID: "n:Message:1", Label: "Message",
		Properties: graph.Properties{"text": "shared data"},
		CreatedAt:  time.Now(),
	})
	bs.Index(&graph.Node{
		ID: "n:SharedMemory:1", Label: "SharedMemory",
		Properties: graph.Properties{"value": "shared data"},
		CreatedAt:  time.Now(),
	})

	// Search only Message label
	results, err := bs.Search("Message", "text", "shared", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, graph.NodeID("n:Message:1"), results[0].NodeID)
}

func TestBleveIgnoresNonIndexedLabels(t *testing.T) {
	bs := newTestSearcher(t)

	node := &graph.Node{
		ID: "n:Agent:1", Label: "Agent",
		Properties: graph.Properties{"text": "agent text"},
		CreatedAt:  time.Now(),
	}
	require.NoError(t, bs.Index(node)) // should be no-op

	results, err := bs.Search("Agent", "text", "agent", 10)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestBleveIgnoresNonStringProperties(t *testing.T) {
	bs := newTestSearcher(t)

	node := &graph.Node{
		ID: "n:Message:1", Label: "Message",
		Properties: graph.Properties{"text": 12345},
		CreatedAt:  time.Now(),
	}
	require.NoError(t, bs.Index(node)) // should be no-op for non-string

	results, err := bs.Search("Message", "text", "12345", 10)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestBleveSearchWithLimit(t *testing.T) {
	bs := newTestSearcher(t)

	for i := range 5 {
		bs.Index(&graph.Node{
			ID:         graph.NodeID(fmt.Sprintf("n:Message:%d", i)),
			Label:      "Message",
			Properties: graph.Properties{"text": "common search term data"},
			CreatedAt:  time.Now(),
		})
	}

	results, err := bs.Search("Message", "text", "common", 2)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
