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

	"github.com/blevesearch/bleve/v2"

	"github.com/scalytics/kafgraph/internal/graph"
)

// TextSearchResult holds a full-text search hit.
type TextSearchResult struct {
	NodeID graph.NodeID
	Score  float64
}

// FullTextSearcher defines the interface for full-text search over nodes.
type FullTextSearcher interface {
	Index(node *graph.Node) error
	Remove(nodeID graph.NodeID) error
	Search(label, property, query string, limit int) ([]TextSearchResult, error)
	Close() error
}

// bleveDoc is the document structure indexed by bleve.
type bleveDoc struct {
	Label    string `json:"label"`
	Property string `json:"property"`
	Text     string `json:"text"`
	NodeID   string `json:"nodeID"`
}

// BleveSearcher implements FullTextSearcher using the bleve library.
type BleveSearcher struct {
	index  bleve.Index
	fields []LabelProperty
}

// LabelProperty identifies a label/property pair to index.
type LabelProperty struct {
	Label    string
	Property string
}

// DefaultIndexedFields returns the default set of label/property pairs to index.
func DefaultIndexedFields() []LabelProperty {
	return []LabelProperty{
		{Label: "Message", Property: "text"},
		{Label: "SharedMemory", Property: "value"},
		{Label: "LearningSignal", Property: "summary"},
		{Label: "Conversation", Property: "description"},
	}
}

// NewBleveSearcher creates a new full-text searcher at the given path.
func NewBleveSearcher(path string, fields []LabelProperty) (*BleveSearcher, error) {
	mapping := bleve.NewIndexMapping()
	idx, err := bleve.New(path, mapping)
	if err != nil {
		return nil, fmt.Errorf("create bleve index: %w", err)
	}
	return &BleveSearcher{index: idx, fields: fields}, nil
}

// Index adds a node's text properties to the full-text index.
func (bs *BleveSearcher) Index(node *graph.Node) error {
	for _, lp := range bs.fields {
		if node.Label != lp.Label {
			continue
		}
		text, ok := node.Properties[lp.Property]
		if !ok {
			continue
		}
		textStr, ok := text.(string)
		if !ok {
			continue
		}
		docID := lp.Label + ":" + lp.Property + ":" + string(node.ID)
		doc := bleveDoc{
			Label:    lp.Label,
			Property: lp.Property,
			Text:     textStr,
			NodeID:   string(node.ID),
		}
		if err := bs.index.Index(docID, doc); err != nil {
			return fmt.Errorf("index node %s: %w", node.ID, err)
		}
	}
	return nil
}

// Remove deletes a node from the full-text index.
func (bs *BleveSearcher) Remove(nodeID graph.NodeID) error {
	for _, lp := range bs.fields {
		docID := lp.Label + ":" + lp.Property + ":" + string(nodeID)
		if err := bs.index.Delete(docID); err != nil {
			return fmt.Errorf("remove node %s: %w", nodeID, err)
		}
	}
	return nil
}

// Search queries the full-text index for nodes matching the query string.
func (bs *BleveSearcher) Search(label, property, queryStr string, limit int) ([]TextSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	query := bleve.NewMatchQuery(queryStr)
	query.SetField("text")
	req := bleve.NewSearchRequestOptions(query, limit, 0, false)
	searchResult, err := bs.index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	prefix := label + ":" + property + ":"
	var results []TextSearchResult
	for _, hit := range searchResult.Hits {
		if len(hit.ID) <= len(prefix) {
			continue
		}
		if hit.ID[:len(prefix)] != prefix {
			continue
		}
		nodeID := graph.NodeID(hit.ID[len(prefix):])
		results = append(results, TextSearchResult{NodeID: nodeID, Score: hit.Score})
	}
	return results, nil
}

// Close closes the bleve index.
func (bs *BleveSearcher) Close() error {
	return bs.index.Close()
}
