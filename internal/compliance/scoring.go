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

// CalculateScore computes a weighted compliance score (0–100) from rule results.
// Each rule result is weighted by severity: critical=3, high=2, medium=1, low=0.5.
// The score is (weighted passes / total weighted) * 100.
func CalculateScore(results []RuleResult) float64 {
	if len(results) == 0 {
		return 100.0
	}

	var weightedPass, totalWeight float64
	for _, r := range results {
		if r.Status == EvalNA {
			continue
		}
		w := r.Severity.Weight()
		totalWeight += w
		if r.Status == EvalPass {
			weightedPass += w
		}
	}

	if totalWeight == 0 {
		return 100.0
	}

	score := (weightedPass / totalWeight) * 100.0
	// Round to one decimal.
	return float64(int(score*10)) / 10
}

// ScoreByModule groups results by module and returns per-module scores.
func ScoreByModule(results []RuleResult, rules []Rule) map[string]float64 {
	ruleModules := map[string]string{}
	for _, r := range rules {
		ruleModules[r.ID()] = r.Module()
	}

	byModule := map[string][]RuleResult{}
	for _, r := range results {
		mod := ruleModules[r.RuleID]
		if mod == "" {
			mod = "unknown"
		}
		byModule[mod] = append(byModule[mod], r)
	}

	scores := map[string]float64{}
	for mod, res := range byModule {
		scores[mod] = CalculateScore(res)
	}
	return scores
}
