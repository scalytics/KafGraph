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
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/scalytics/kafgraph/internal/graph"
)

const vectorPrefix = "v:"

// VectorSearchResult holds a search hit with its similarity score.
type VectorSearchResult struct {
	NodeID graph.NodeID
	Score  float64
}

// VectorSearcher defines the interface for vector similarity search.
type VectorSearcher interface {
	Index(label, property string, nodeID graph.NodeID, vector []float32) error
	Remove(nodeID graph.NodeID) error
	Search(label, property string, query []float32, k int) ([]VectorSearchResult, error)
	Close() error
}

// BruteForceVectorSearcher implements VectorSearcher using BadgerDB storage
// and brute-force cosine similarity.
type BruteForceVectorSearcher struct {
	db *badger.DB
	mu sync.RWMutex
}

// NewBruteForceVectorSearcher creates a new vector searcher backed by BadgerDB.
func NewBruteForceVectorSearcher(db *badger.DB) *BruteForceVectorSearcher {
	return &BruteForceVectorSearcher{db: db}
}

func vectorKey(label, property string, nodeID graph.NodeID) []byte {
	return []byte(vectorPrefix + label + ":" + property + ":" + string(nodeID))
}

// Index stores a vector for the given node, label, and property.
func (vs *BruteForceVectorSearcher) Index(label, property string, nodeID graph.NodeID, vector []float32) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	data := encodeFloat32s(vector)
	return vs.db.Update(func(txn *badger.Txn) error {
		return txn.Set(vectorKey(label, property, nodeID), data)
	})
}

// Remove deletes all vectors for the given node.
func (vs *BruteForceVectorSearcher) Remove(nodeID graph.NodeID) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	// Scan for all keys containing this nodeID
	suffix := ":" + string(nodeID)
	var keysToDelete [][]byte
	err := vs.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(vectorPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if strings.HasSuffix(key, suffix) {
				keysToDelete = append(keysToDelete, it.Item().KeyCopy(nil))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(keysToDelete) == 0 {
		return nil
	}
	return vs.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

// Search finds the k most similar vectors using brute-force cosine similarity.
func (vs *BruteForceVectorSearcher) Search(label, property string, query []float32, k int) ([]VectorSearchResult, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if k <= 0 {
		return nil, fmt.Errorf("k must be positive")
	}

	prefix := vectorPrefix + label + ":" + property + ":"
	var results []VectorSearchResult

	err := vs.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())
			nodeID := graph.NodeID(strings.TrimPrefix(key, prefix))

			var vec []float32
			err := item.Value(func(val []byte) error {
				vec = decodeFloat32s(val)
				return nil
			})
			if err != nil {
				return err
			}

			score := cosineSimilarity(query, vec)
			results = append(results, VectorSearchResult{NodeID: nodeID, Score: score})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > k {
		results = results[:k]
	}
	return results, nil
}

// Close is a no-op; the underlying DB is managed externally.
func (vs *BruteForceVectorSearcher) Close() error {
	return nil
}

func encodeFloat32s(v []float32) []byte {
	buf := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeFloat32s(data []byte) []float32 {
	n := len(data) / 4
	v := make([]float32, n)
	for i := range n {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return v
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
