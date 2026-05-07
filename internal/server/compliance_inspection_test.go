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

package server

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func TestComplianceInspectionCRUD(t *testing.T) {
	ts, _, g := setupComplianceTestServer(t)

	scope, err := g.CreateNode("ProcessingActivity", graph.Properties{"name": "Payroll", "status": "active"})
	require.NoError(t, err)
	scan, err := g.CreateNode("ComplianceScan", graph.Properties{"scanId": "scan-1"})
	require.NoError(t, err)

	createResp := complianceRequest(t, ts.URL, http.MethodPost, "/api/v2/compliance/inspections", `{
		"properties":{"title":"Quarterly Review","inspectorId":"ins-1"},
		"scopeNodeIds":["`+string(scope.ID)+`"],
		"scanId":"scan-1"
	}`)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	inspection := decodeBody(t, createResp)
	inspectionID := inspection["id"].(string)

	listResp := complianceRequest(t, ts.URL, http.MethodGet, "/api/v2/compliance/inspections?status=draft", "")
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	listBody := decodeBody(t, listResp)
	assert.Equal(t, float64(1), listBody["total"])

	findingResp := complianceRequest(t, ts.URL, http.MethodPost, "/api/v2/compliance/inspections/"+inspectionID+"/findings", `{
		"properties":{"title":"Missing evidence"},
		"affectedNodeIds":["`+string(scope.ID)+`"]
	}`)
	require.Equal(t, http.StatusCreated, findingResp.StatusCode)
	finding := decodeBody(t, findingResp)
	findingID := finding["id"].(string)

	remediationResp := complianceRequest(t, ts.URL, http.MethodPost, "/api/v2/compliance/findings/"+findingID+"/remediation", `{"title":"Attach evidence"}`)
	require.Equal(t, http.StatusCreated, remediationResp.StatusCode)
	remediation := decodeBody(t, remediationResp)
	remediationID := remediation["id"].(string)

	updateFindingResp := complianceRequest(t, ts.URL, http.MethodPut, "/api/v2/compliance/findings/"+findingID, `{"status":"remediated","title":"Resolved"}`)
	require.Equal(t, http.StatusOK, updateFindingResp.StatusCode)
	_ = decodeBody(t, updateFindingResp)

	updateRemediationResp := complianceRequest(t, ts.URL, http.MethodPut, "/api/v2/compliance/remediation/"+remediationID, `{"status":"completed","verifiedBy":"lead-1"}`)
	require.Equal(t, http.StatusOK, updateRemediationResp.StatusCode)
	_ = decodeBody(t, updateRemediationResp)

	detailResp := complianceRequest(t, ts.URL, http.MethodGet, "/api/v2/compliance/inspections/"+inspectionID, "")
	require.Equal(t, http.StatusOK, detailResp.StatusCode)
	detail := decodeBody(t, detailResp)
	assert.Len(t, detail["findings"], 1)
	assert.Len(t, detail["scope"], 1)
	assert.NotNil(t, detail["basedOnScan"])

	findingDetailResp := complianceRequest(t, ts.URL, http.MethodGet, "/api/v2/compliance/findings/"+findingID, "")
	require.Equal(t, http.StatusOK, findingDetailResp.StatusCode)
	findingDetail := decodeBody(t, findingDetailResp)
	assert.Len(t, findingDetail["remediations"], 1)
	assert.Len(t, findingDetail["affected"], 1)

	updateInspectionResp := complianceRequest(t, ts.URL, http.MethodPut, "/api/v2/compliance/inspections/"+inspectionID, `{"status":"review"}`)
	require.Equal(t, http.StatusOK, updateInspectionResp.StatusCode)
	_ = decodeBody(t, updateInspectionResp)

	signOffResp := complianceRequest(t, ts.URL, http.MethodPost, "/api/v2/compliance/inspections/"+inspectionID+"/sign-off", `{"approverId":"lead-1"}`)
	require.Equal(t, http.StatusOK, signOffResp.StatusCode)
	signOffBody := decodeBody(t, signOffResp)
	assert.Equal(t, "signed_off", signOffBody["status"])

	eventsResp := complianceRequest(t, ts.URL, http.MethodGet, "/api/v2/compliance/events?limit=2", "")
	require.Equal(t, http.StatusOK, eventsResp.StatusCode)
	events := decodeBody(t, eventsResp)
	assert.Len(t, events["events"], 2)
	assert.True(t, events["total"].(float64) >= 2)

	_, err = g.GetNode(scan.ID)
	require.NoError(t, err)
}
