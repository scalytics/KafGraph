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

// stopwords is a set of common English words that carry little topical weight.
// Used by TF-IDF to filter noise. ~175 entries.
var stopwords = map[string]bool{
	"a": true, "about": true, "above": true, "after": true, "again": true,
	"against": true, "all": true, "am": true, "an": true, "and": true,
	"any": true, "are": true, "as": true, "at": true, "be": true,
	"because": true, "been": true, "before": true, "being": true, "below": true,
	"between": true, "both": true, "but": true, "by": true, "can": true,
	"could": true, "did": true, "do": true, "does": true, "doing": true,
	"don": true, "down": true, "during": true, "each": true, "few": true,
	"for": true, "from": true, "further": true, "get": true, "got": true,
	"had": true, "has": true, "have": true, "having": true, "he": true,
	"her": true, "here": true, "hers": true, "herself": true, "him": true,
	"himself": true, "his": true, "how": true, "i": true, "if": true,
	"in": true, "into": true, "is": true, "it": true, "its": true,
	"itself": true, "just": true, "let": true, "like": true, "ll": true,
	"may": true, "me": true, "might": true, "more": true, "most": true,
	"must": true, "my": true, "myself": true, "no": true, "nor": true,
	"not": true, "now": true, "of": true, "off": true, "on": true,
	"once": true, "only": true, "or": true, "other": true, "our": true,
	"ours": true, "ourselves": true, "out": true, "over": true, "own": true,
	"re": true, "s": true, "same": true, "shall": true, "she": true,
	"should": true, "so": true, "some": true, "such": true, "t": true,
	"than": true, "that": true, "the": true, "their": true, "theirs": true,
	"them": true, "themselves": true, "then": true, "there": true, "these": true,
	"they": true, "this": true, "those": true, "through": true, "to": true,
	"too": true, "under": true, "until": true, "up": true, "us": true,
	"ve": true, "very": true, "was": true, "we": true, "were": true,
	"what": true, "when": true, "where": true, "which": true, "while": true,
	"who": true, "whom": true, "why": true, "will": true, "with": true,
	"won": true, "would": true, "you": true, "your": true, "yours": true,
	"yourself": true, "yourselves": true,
	// Additional common words
	"also": true, "back": true, "even": true, "go": true, "going": true,
	"gone": true, "good": true, "great": true,
	"know": true, "make": true, "much": true, "need": true,
	"new": true, "one": true, "really": true, "right": true, "said": true,
	"say": true, "see": true, "since": true, "still": true, "take": true,
	"tell": true, "thing": true, "think": true, "time": true, "two": true,
	"use": true, "used": true, "using": true, "want": true, "way": true,
	"well": true, "work": true, "year": true,
}

// isStopword returns true if the token is a stopword or too short to be meaningful.
func isStopword(token string) bool {
	return len(token) < 3 || stopwords[token]
}
