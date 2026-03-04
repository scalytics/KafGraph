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

import "fmt"

// YAMLRuleDef is the on-disk representation of a YAML-defined compliance rule.
type YAMLRuleDef struct {
	RuleID       string `yaml:"id"`
	FrameworkStr string `yaml:"framework"`
	ModuleStr    string `yaml:"module"`
	ArticleStr   string `yaml:"article"`
	TitleStr     string `yaml:"title"`
	SeverityStr  string `yaml:"severity"`
	Check        struct {
		NodeLabel          string   `yaml:"nodeLabel"`
		RequiredProperties []string `yaml:"requiredProperties"`
		Condition          string   `yaml:"condition"`
		EdgeLabel          string   `yaml:"edgeLabel"`
	} `yaml:"check"`
}

// ToRule converts a YAMLRuleDef into a Rule implementation.
func (d *YAMLRuleDef) ToRule() Rule {
	return &yamlRule{def: d}
}

type yamlRule struct {
	def *YAMLRuleDef
}

func (r *yamlRule) ID() string        { return r.def.RuleID }
func (r *yamlRule) Framework() Framework { return Framework(r.def.FrameworkStr) }
func (r *yamlRule) Module() string     { return r.def.ModuleStr }
func (r *yamlRule) Article() string    { return r.def.ArticleStr }
func (r *yamlRule) Title() string      { return r.def.TitleStr }

func (r *yamlRule) Severity() Severity {
	switch Severity(r.def.SeverityStr) {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
		return Severity(r.def.SeverityStr)
	default:
		return SeverityMedium
	}
}

func (r *yamlRule) Evaluate(g GraphQuerier) ([]RuleResult, error) {
	c := r.def.Check
	if c.NodeLabel == "" {
		return nil, fmt.Errorf("yaml rule %s: nodeLabel is required", r.def.RuleID)
	}

	switch c.Condition {
	case "property_not_empty", "":
		return r.evaluatePropertyNotEmpty(g)
	case "has_edge":
		return r.evaluateHasEdge(g)
	default:
		return nil, fmt.Errorf("yaml rule %s: unknown condition %q", r.def.RuleID, c.Condition)
	}
}

func (r *yamlRule) evaluatePropertyNotEmpty(g GraphQuerier) ([]RuleResult, error) {
	nodes, err := g.NodesByLabel(r.def.Check.NodeLabel)
	if err != nil {
		return nil, err
	}

	var results []RuleResult
	for _, n := range nodes {
		allPresent := true
		missing := ""
		for _, prop := range r.def.Check.RequiredProperties {
			val, _ := n.Properties[prop].(string)
			if val == "" {
				allPresent = false
				missing = prop
				break
			}
		}
		if allPresent {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalPass, NodeID: n.ID,
				Details:  "All required properties present",
				Severity: r.Severity(),
			})
		} else {
			results = append(results, RuleResult{
				RuleID: r.ID(), Status: EvalFail, NodeID: n.ID,
				Details:  fmt.Sprintf("Missing property: %s", missing),
				Severity: r.Severity(),
			})
		}
	}
	return results, nil
}

func (r *yamlRule) evaluateHasEdge(g GraphQuerier) ([]RuleResult, error) {
	return checkHasEdge(g, r.ID(), r.Severity(), r.def.Check.NodeLabel, r.def.Check.EdgeLabel)
}
