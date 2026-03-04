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
	"testing"
)

func TestTFIDFCorpus_EmptyCorpus(t *testing.T) {
	c := NewTFIDFCorpus()
	kw := c.TopKeywords("hello world", 5)
	if kw != nil {
		t.Errorf("expected nil keywords from empty corpus, got %v", kw)
	}
}

func TestTFIDFCorpus_SingleDocument(t *testing.T) {
	c := NewTFIDFCorpus()
	c.AddDocument("distributed systems are built for fault tolerance")
	if c.DocCount() != 1 {
		t.Fatalf("expected 1 doc, got %d", c.DocCount())
	}

	kw := c.TopKeywords("distributed systems are built for fault tolerance", 3)
	if len(kw) == 0 {
		t.Fatal("expected keywords from single document")
	}
	for _, k := range kw {
		if k.Score <= 0 {
			t.Errorf("expected positive score for %q, got %f", k.Term, k.Score)
		}
	}
}

func TestTFIDFCorpus_MultipleDocs(t *testing.T) {
	c := NewTFIDFCorpus()
	c.AddDocument("agent coordination in distributed systems")
	c.AddDocument("distributed systems require fault tolerance")
	c.AddDocument("machine learning models for natural language processing")

	// "distributed" appears in 2/3 docs, "machine" in 1/3.
	// For the ML doc, "machine" should score higher than "distributed".
	kw := c.TopKeywords("machine learning models for natural language processing", 5)
	if len(kw) == 0 {
		t.Fatal("expected keywords")
	}

	// First keyword should be from ML domain (higher IDF).
	found := false
	for _, k := range kw {
		if k.Term == "machine" || k.Term == "learning" || k.Term == "models" || k.Term == "natural" || k.Term == "language" || k.Term == "processing" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ML-related term in top keywords, got: %v", kw)
	}
}

func TestTFIDFCorpus_StopwordsFiltered(t *testing.T) {
	c := NewTFIDFCorpus()
	c.AddDocument("the quick brown fox jumps over the lazy dog")

	kw := c.TopKeywords("the quick brown fox jumps over the lazy dog", 10)
	for _, k := range kw {
		if isStopword(k.Term) {
			t.Errorf("stopword %q should have been filtered", k.Term)
		}
	}
}

func TestTFIDFCorpus_TopKeywordsLimit(t *testing.T) {
	c := NewTFIDFCorpus()
	c.AddDocument("alpha beta gamma delta epsilon zeta eta theta")

	kw := c.TopKeywords("alpha beta gamma delta epsilon zeta eta theta", 3)
	if len(kw) != 3 {
		t.Errorf("expected 3 keywords, got %d", len(kw))
	}
}

func TestTFIDFCorpus_EmptyText(t *testing.T) {
	c := NewTFIDFCorpus()
	c.AddDocument("some content here")

	kw := c.TopKeywords("", 5)
	if kw != nil {
		t.Errorf("expected nil keywords from empty text, got %v", kw)
	}
}

func TestTokenizeClean(t *testing.T) {
	tokens := tokenizeClean("Hello, World! This is a Test-Case for tokenization.")
	for _, tok := range tokens {
		if isStopword(tok) {
			t.Errorf("stopword %q should not be in cleaned tokens", tok)
		}
	}
	// "test-case" should be kept (has hyphens).
	found := false
	for _, tok := range tokens {
		if tok == "test-case" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'test-case' in tokens, got: %v", tokens)
	}
}

func TestExtractBigrams(t *testing.T) {
	bigrams := extractBigrams("distributed systems agent coordination protocol")
	if len(bigrams) == 0 {
		t.Fatal("expected bigrams")
	}
	// Check no duplicates.
	seen := make(map[string]bool)
	for _, bg := range bigrams {
		if seen[bg] {
			t.Errorf("duplicate bigram: %q", bg)
		}
		seen[bg] = true
	}
}

func TestExtractTrigrams(t *testing.T) {
	trigrams := extractTrigrams("distributed systems agent coordination protocol design")
	if len(trigrams) == 0 {
		t.Fatal("expected trigrams")
	}
	seen := make(map[string]bool)
	for _, tg := range trigrams {
		if seen[tg] {
			t.Errorf("duplicate trigram: %q", tg)
		}
		seen[tg] = true
	}
}
