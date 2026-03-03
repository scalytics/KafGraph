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
	"context"

	"github.com/scalytics/kafgraph/internal/brain"
)

// BrainAdapter adapts CycleRunner to the brain.ReflectionRunner interface.
type BrainAdapter struct {
	runner *CycleRunner
}

// NewBrainAdapter creates an adapter that satisfies brain.ReflectionRunner.
func NewBrainAdapter(runner *CycleRunner) *BrainAdapter {
	return &BrainAdapter{runner: runner}
}

// ExecuteForBrain implements brain.ReflectionRunner.
func (ba *BrainAdapter) ExecuteForBrain(ctx context.Context, agentID string, windowHours int) (*brain.ReflectCycleResult, error) {
	result, err := ba.runner.ExecuteForBrain(ctx, agentID, windowHours)
	if err != nil {
		return nil, err
	}

	signals := make([]brain.NodeSummary, len(result.LearningSignals))
	for i, sig := range result.LearningSignals {
		signals[i] = brain.NodeSummary{
			NodeID:  string(sig.NodeID),
			Type:    sig.Label,
			Summary: sig.Summary,
		}
	}

	return &brain.ReflectCycleResult{
		CycleID:         string(result.CycleID),
		LearningSignals: signals,
		Summary:         result.Summary,
	}, nil
}
