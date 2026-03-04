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

package compliance

import (
	"math"
	"testing"
)

func TestCalculateScore_Empty(t *testing.T) {
	score := CalculateScore(nil)
	if score != 100.0 {
		t.Fatalf("expected 100.0 for empty results, got %v", score)
	}
}

func TestCalculateScore_AllPass(t *testing.T) {
	results := []RuleResult{
		{RuleID: "R1", Status: EvalPass, Severity: SeverityCritical},
		{RuleID: "R2", Status: EvalPass, Severity: SeverityHigh},
	}
	score := CalculateScore(results)
	if score != 100.0 {
		t.Fatalf("expected 100.0 for all pass, got %v", score)
	}
}

func TestCalculateScore_AllFail(t *testing.T) {
	results := []RuleResult{
		{RuleID: "R1", Status: EvalFail, Severity: SeverityCritical},
		{RuleID: "R2", Status: EvalFail, Severity: SeverityHigh},
	}
	score := CalculateScore(results)
	if score != 0.0 {
		t.Fatalf("expected 0.0 for all fail, got %v", score)
	}
}

func TestCalculateScore_Mixed(t *testing.T) {
	results := []RuleResult{
		{RuleID: "R1", Status: EvalPass, Severity: SeverityCritical}, // weight 3
		{RuleID: "R2", Status: EvalFail, Severity: SeverityLow},     // weight 0.5
	}
	// Expected: 3 / 3.5 * 100 = 85.7
	score := CalculateScore(results)
	if math.Abs(score-85.7) > 0.1 {
		t.Fatalf("expected ~85.7, got %v", score)
	}
}

func TestCalculateScore_NAIgnored(t *testing.T) {
	results := []RuleResult{
		{RuleID: "R1", Status: EvalPass, Severity: SeverityCritical},
		{RuleID: "R2", Status: EvalNA, Severity: SeverityCritical},
	}
	score := CalculateScore(results)
	if score != 100.0 {
		t.Fatalf("expected 100.0 (NA ignored), got %v", score)
	}
}

func TestCalculateScore_AllNA(t *testing.T) {
	results := []RuleResult{
		{RuleID: "R1", Status: EvalNA, Severity: SeverityCritical},
	}
	score := CalculateScore(results)
	if score != 100.0 {
		t.Fatalf("expected 100.0 for all NA, got %v", score)
	}
}
