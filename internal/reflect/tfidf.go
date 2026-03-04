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
	"math"
	"sort"
	"strings"
	"unicode"
)

// TFIDFCorpus computes TF-IDF scores over a document corpus.
type TFIDFCorpus struct {
	docCount int
	// docFreq tracks how many documents each term appears in.
	docFreq map[string]int
}

// NewTFIDFCorpus creates an empty corpus.
func NewTFIDFCorpus() *TFIDFCorpus {
	return &TFIDFCorpus{
		docFreq: make(map[string]int),
	}
}

// AddDocument registers a document's term set in the corpus.
// Call once per document before computing scores.
func (c *TFIDFCorpus) AddDocument(text string) {
	c.docCount++
	seen := make(map[string]bool)
	for _, token := range tokenizeClean(text) {
		if !seen[token] {
			seen[token] = true
			c.docFreq[token]++
		}
	}
}

// TopKeywords returns the top-N keywords from text ranked by TF-IDF score.
// The corpus must have documents added before calling this.
func (c *TFIDFCorpus) TopKeywords(text string, n int) []Keyword {
	if c.docCount == 0 || n <= 0 {
		return nil
	}

	tokens := tokenizeClean(text)
	if len(tokens) == 0 {
		return nil
	}

	// Compute term frequency in this document.
	tf := make(map[string]int)
	for _, t := range tokens {
		tf[t]++
	}

	// Compute TF-IDF for each term.
	type scored struct {
		term  string
		score float64
	}
	var results []scored
	totalTerms := float64(len(tokens))
	for term, count := range tf {
		df := c.docFreq[term]
		if df == 0 {
			df = 1
		}
		// Use smoothed IDF: log((N+1)/(df)) to ensure single-doc corpus
		// still produces positive scores for non-universal terms.
		idf := math.Log(float64(c.docCount+1) / float64(df))
		tfidf := (float64(count) / totalTerms) * idf
		results = append(results, scored{term: term, score: tfidf})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if n > len(results) {
		n = len(results)
	}

	keywords := make([]Keyword, n)
	for i := 0; i < n; i++ {
		keywords[i] = Keyword{Term: results[i].term, Score: results[i].score}
	}
	return keywords
}

// DocCount returns the number of documents in the corpus.
func (c *TFIDFCorpus) DocCount() int {
	return c.docCount
}

// tokenizeClean splits text into lowercase tokens, strips punctuation,
// and removes stopwords and short tokens.
func tokenizeClean(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
	var result []string
	for _, w := range words {
		w = strings.Trim(w, "-")
		if w != "" && !isStopword(w) {
			result = append(result, w)
		}
	}
	return result
}

// extractBigrams returns all unique bigrams from cleaned tokens.
func extractBigrams(text string) []string {
	tokens := tokenizeClean(text)
	if len(tokens) < 2 {
		return nil
	}
	seen := make(map[string]bool)
	var bigrams []string
	for i := 0; i < len(tokens)-1; i++ {
		bg := tokens[i] + " " + tokens[i+1]
		if !seen[bg] {
			seen[bg] = true
			bigrams = append(bigrams, bg)
		}
	}
	return bigrams
}

// extractTrigrams returns all unique trigrams from cleaned tokens.
func extractTrigrams(text string) []string {
	tokens := tokenizeClean(text)
	if len(tokens) < 3 {
		return nil
	}
	seen := make(map[string]bool)
	var trigrams []string
	for i := 0; i < len(tokens)-2; i++ {
		tg := tokens[i] + " " + tokens[i+1] + " " + tokens[i+2]
		if !seen[tg] {
			seen[tg] = true
			trigrams = append(trigrams, tg)
		}
	}
	return trigrams
}
