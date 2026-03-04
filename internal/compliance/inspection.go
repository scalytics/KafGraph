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
	"fmt"
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// InspectionStatus tracks the lifecycle of an inspection.
type InspectionStatus string

const (
	InspectionDraft      InspectionStatus = "draft"
	InspectionInProgress InspectionStatus = "in_progress"
	InspectionReview     InspectionStatus = "review"
	InspectionSignedOff  InspectionStatus = "signed_off"
	InspectionClosed     InspectionStatus = "closed"
)

// FindingStatus tracks the lifecycle of an inspection finding.
type FindingStatus string

const (
	FindingOpen       FindingStatus = "open"
	FindingRemediated FindingStatus = "remediated"
	FindingWaived     FindingStatus = "waived"
	FindingAccepted   FindingStatus = "accepted"
)

// RemediationStatus tracks the lifecycle of a remediation action.
type RemediationStatus string

const (
	RemediationPending    RemediationStatus = "pending"
	RemediationInProgress RemediationStatus = "in_progress"
	RemediationCompleted  RemediationStatus = "completed"
	RemediationOverdue    RemediationStatus = "overdue"
)

// CreateInspection creates an Inspection node and links it to scope entities.
func CreateInspection(g *graph.Graph, props graph.Properties, scopeNodeIDs []string, scanID string) (*graph.Node, error) {
	if props["status"] == nil {
		props["status"] = string(InspectionDraft)
	}
	props["createdAt"] = time.Now().UTC().Format(time.RFC3339)

	node, err := g.CreateNode("Inspection", props)
	if err != nil {
		return nil, fmt.Errorf("create inspection node: %w", err)
	}

	// Link to scope entities.
	for _, id := range scopeNodeIDs {
		_, _ = g.CreateEdge("INSPECTS", node.ID, graph.NodeID(id), nil)
	}

	// Link to baseline scan if provided.
	if scanID != "" {
		scans, _ := g.NodesByLabel("ComplianceScan")
		for _, s := range scans {
			if sID, ok := s.Properties["scanId"].(string); ok && sID == scanID {
				_, _ = g.CreateEdge("BASED_ON", node.ID, s.ID, nil)
				break
			}
		}
	}

	actor, _ := props["inspectorId"].(string)
	LogEvent(g, "inspection_created", actor, fmt.Sprintf("Inspection %s created", string(node.ID)), string(node.ID))
	return node, nil
}

// SignOffInspection transitions an inspection to signed_off status.
// All findings must be remediated, waived, or accepted.
func SignOffInspection(g *graph.Graph, inspectionID graph.NodeID, approverID string) error {
	node, err := g.GetNode(inspectionID)
	if err != nil {
		return fmt.Errorf("get inspection: %w", err)
	}

	// Check all findings are resolved.
	edges, _ := g.Neighbors(inspectionID)
	for _, e := range edges {
		if e.Label != "HAS_FINDING" {
			continue
		}
		finding, err := g.GetNode(e.ToID)
		if err != nil {
			continue
		}
		status, _ := finding.Properties["status"].(string)
		if status == string(FindingOpen) {
			return fmt.Errorf("cannot sign off: finding %s is still open", string(finding.ID))
		}
	}

	_, err = g.UpsertNode(inspectionID, node.Label, graph.Properties{
		"status":     string(InspectionSignedOff),
		"approverId": approverID,
		"signedOffAt": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("update inspection status: %w", err)
	}

	// Link approver.
	agents, _ := g.NodesByLabel("Agent")
	for _, a := range agents {
		if aID, ok := a.Properties["agentId"].(string); ok && aID == approverID {
			_, _ = g.CreateEdge("APPROVED_BY", inspectionID, a.ID, nil)
			break
		}
	}

	LogEvent(g, "inspection_signed_off", approverID, fmt.Sprintf("Inspection %s signed off", string(inspectionID)), string(inspectionID))
	return nil
}

// CreateFinding adds a finding to an inspection.
func CreateFinding(g *graph.Graph, inspectionID graph.NodeID, props graph.Properties, affectedNodeIDs []string) (*graph.Node, error) {
	if props["status"] == nil {
		props["status"] = string(FindingOpen)
	}
	props["createdAt"] = time.Now().UTC().Format(time.RFC3339)

	node, err := g.CreateNode("InspectionFinding", props)
	if err != nil {
		return nil, fmt.Errorf("create finding: %w", err)
	}

	_, _ = g.CreateEdge("HAS_FINDING", inspectionID, node.ID, nil)

	for _, id := range affectedNodeIDs {
		_, _ = g.CreateEdge("AFFECTS", node.ID, graph.NodeID(id), nil)
	}

	LogEvent(g, "finding_opened", "", fmt.Sprintf("Finding %s opened for inspection %s", string(node.ID), string(inspectionID)), string(node.ID))
	return node, nil
}

// CreateRemediation adds a remediation action to a finding.
func CreateRemediation(g *graph.Graph, findingID graph.NodeID, props graph.Properties) (*graph.Node, error) {
	if props["status"] == nil {
		props["status"] = string(RemediationPending)
	}
	props["createdAt"] = time.Now().UTC().Format(time.RFC3339)

	node, err := g.CreateNode("RemediationAction", props)
	if err != nil {
		return nil, fmt.Errorf("create remediation: %w", err)
	}

	_, _ = g.CreateEdge("REMEDIATED_BY", findingID, node.ID, nil)

	LogEvent(g, "remediation_created", "", fmt.Sprintf("Remediation %s created for finding %s", string(node.ID), string(findingID)), string(node.ID))
	return node, nil
}

// LogEvent creates an immutable ComplianceEvent audit log entry.
func LogEvent(g *graph.Graph, eventType, actor, details, relatedNodeID string) {
	if g == nil {
		return
	}
	props := graph.Properties{
		"eventType": eventType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"actor":     actor,
		"details":   details,
	}
	if relatedNodeID != "" {
		props["relatedNodeId"] = relatedNodeID
	}
	node, err := g.CreateNode("ComplianceEvent", props)
	if err != nil {
		return
	}
	if relatedNodeID != "" {
		_, _ = g.CreateEdge("RELATES_TO", node.ID, graph.NodeID(relatedNodeID), nil)
	}
}
