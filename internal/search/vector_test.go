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
	"testing"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *badger.DB {
	t.Helper()
	opts := badger.DefaultOptions("").WithInMemory(true).WithLoggingLevel(badger.WARNING)
	db, err := badger.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestVectorIndex(t *testing.T) {
	vs := NewBruteForceVectorSearcher(newTestDB(t))

	err := vs.Index("Agent", "embedding", "n:Agent:alice", []float32{1.0, 0.0, 0.0})
	require.NoError(t, err)
}

func TestVectorSearchExact(t *testing.T) {
	vs := NewBruteForceVectorSearcher(newTestDB(t))

	vs.Index("Agent", "embedding", "n:Agent:alice", []float32{1.0, 0.0, 0.0})
	vs.Index("Agent", "embedding", "n:Agent:bob", []float32{0.0, 1.0, 0.0})

	results, err := vs.Search("Agent", "embedding", []float32{1.0, 0.0, 0.0}, 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "n:Agent:alice", string(results[0].NodeID))
	assert.InDelta(t, 1.0, results[0].Score, 0.001)
}

func TestVectorSearchTopK(t *testing.T) {
	vs := NewBruteForceVectorSearcher(newTestDB(t))

	vs.Index("Agent", "embedding", "n:1", []float32{1.0, 0.0})
	vs.Index("Agent", "embedding", "n:2", []float32{0.9, 0.1})
	vs.Index("Agent", "embedding", "n:3", []float32{0.0, 1.0})

	results, err := vs.Search("Agent", "embedding", []float32{1.0, 0.0}, 2)
	require.NoError(t, err)
	require.Len(t, results, 2)
	// First result should be the most similar
	assert.Equal(t, "n:1", string(results[0].NodeID))
}

func TestVectorSearchEmpty(t *testing.T) {
	vs := NewBruteForceVectorSearcher(newTestDB(t))

	results, err := vs.Search("Agent", "embedding", []float32{1.0, 0.0}, 5)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestVectorSearchInvalidK(t *testing.T) {
	vs := NewBruteForceVectorSearcher(newTestDB(t))

	_, err := vs.Search("Agent", "embedding", []float32{1.0}, 0)
	assert.Error(t, err)
}

func TestVectorRemove(t *testing.T) {
	vs := NewBruteForceVectorSearcher(newTestDB(t))

	vs.Index("Agent", "embedding", "n:Agent:alice", []float32{1.0, 0.0})
	require.NoError(t, vs.Remove("n:Agent:alice"))

	results, err := vs.Search("Agent", "embedding", []float32{1.0, 0.0}, 5)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestVectorSearchLabelIsolation(t *testing.T) {
	vs := NewBruteForceVectorSearcher(newTestDB(t))

	vs.Index("Agent", "embedding", "n:1", []float32{1.0, 0.0})
	vs.Index("Message", "embedding", "n:2", []float32{1.0, 0.0})

	results, err := vs.Search("Agent", "embedding", []float32{1.0, 0.0}, 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "n:1", string(results[0].NodeID))
}

func TestCosineSimilarity(t *testing.T) {
	assert.InDelta(t, 1.0, cosineSimilarity([]float32{1, 0}, []float32{1, 0}), 0.001)
	assert.InDelta(t, 0.0, cosineSimilarity([]float32{1, 0}, []float32{0, 1}), 0.001)
	assert.InDelta(t, -1.0, cosineSimilarity([]float32{1, 0}, []float32{-1, 0}), 0.001)
	assert.Equal(t, 0.0, cosineSimilarity([]float32{}, []float32{}))
	assert.Equal(t, 0.0, cosineSimilarity([]float32{1}, []float32{1, 2}))
}
