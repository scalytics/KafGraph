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
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
)

func complianceRequest(t *testing.T, tsURL, method, path, body string) *http.Response {
	t.Helper()
	var reqBody *strings.Reader
	if body == "" {
		reqBody = strings.NewReader("")
	} else {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, tsURL+path, reqBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

func TestComplianceDataFlowCRUDAndValidate(t *testing.T) {
	ts, _, g := setupComplianceTestServer(t)

	src, err := g.CreateNode("ProcessingActivity", graph.Properties{"name": "HR Intake", "status": "active"})
	require.NoError(t, err)
	dst, err := g.CreateNode("ProcessingActivity", graph.Properties{"name": "Payroll", "status": "active"})
	require.NoError(t, err)
	proc, err := g.CreateNode("DataProcessor", graph.Properties{"name": "Vendor X"})
	require.NoError(t, err)
	cat, err := g.CreateNode("DataCategory", graph.Properties{"name": "Employee Data", "isSpecial": false})
	require.NoError(t, err)
	basis, err := g.CreateNode("LegalBasis", graph.Properties{"name": "Contract"})
	require.NoError(t, err)
	_, err = g.CreateNode("ProcessingActivity", graph.Properties{"name": "Orphan Activity", "status": "active"})
	require.NoError(t, err)

	createResp := complianceRequest(t, ts.URL, http.MethodPost, "/api/v2/compliance/gdpr/data-flows", `{
		"properties":{"name":"HR -> Payroll","transferType":"internal","legalBasis":"contract"},
		"fromActivityId":"`+string(src.ID)+`",
		"toActivityId":"`+string(dst.ID)+`",
		"toProcessorId":"`+string(proc.ID)+`",
		"categoryIds":["`+string(cat.ID)+`"],
		"legalBasisId":"`+string(basis.ID)+`"
	}`)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	created := decodeBody(t, createResp)
	flowID, ok := created["id"].(string)
	require.True(t, ok)
	assert.Equal(t, "DataFlow", created["label"])
	require.NotEmpty(t, created["properties"].(map[string]any)["createdAt"])

	listResp := complianceRequest(t, ts.URL, http.MethodGet, "/api/v2/compliance/gdpr/data-flows", "")
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	listBody := decodeBody(t, listResp)
	assert.Equal(t, float64(1), listBody["total"])
	items := listBody["items"].([]any)
	item := items[0].(map[string]any)
	assert.Equal(t, "HR -> Payroll", item["properties"].(map[string]any)["name"])
	assert.Contains(t, item["fromNames"], "HR Intake")
	assert.Contains(t, item["toNames"], "Payroll")
	assert.Contains(t, item["toNames"], "Vendor X")
	assert.Contains(t, item["categoryNames"], "Employee Data")

	detailResp := complianceRequest(t, ts.URL, http.MethodGet, "/api/v2/compliance/gdpr/data-flows/"+flowID, "")
	require.Equal(t, http.StatusOK, detailResp.StatusCode)
	detailBody := decodeBody(t, detailResp)
	assert.Len(t, detailBody["edges"], 6)

	updateResp := complianceRequest(t, ts.URL, http.MethodPut, "/api/v2/compliance/gdpr/data-flows/"+flowID, `{"name":"HR -> Payroll v2","legalBasis":"contract"}`)
	require.Equal(t, http.StatusOK, updateResp.StatusCode)
	updateBody := decodeBody(t, updateResp)
	assert.Equal(t, "HR -> Payroll v2", updateBody["properties"].(map[string]any)["name"])
	require.NotEmpty(t, updateBody["properties"].(map[string]any)["updatedAt"])

	mapResp := complianceRequest(t, ts.URL, http.MethodGet, "/api/v2/compliance/gdpr/data-flows/map", "")
	require.Equal(t, http.StatusOK, mapResp.StatusCode)
	mapBody := decodeBody(t, mapResp)
	assert.NotEmpty(t, mapBody["nodes"])
	assert.NotEmpty(t, mapBody["edges"])

	validateResp := complianceRequest(t, ts.URL, http.MethodPost, "/api/v2/compliance/gdpr/data-flows/validate", `{"inspectionId":"insp-1"}`)
	require.Equal(t, http.StatusOK, validateResp.StatusCode)
	validateBody := decodeBody(t, validateResp)
	assert.Equal(t, float64(3), validateBody["total"])
	assert.Equal(t, float64(1), validateBody["pass"])
	assert.Equal(t, float64(2), validateBody["warnings"])
	assert.Equal(t, float64(0), validateBody["fail"])

	validations, err := g.NodesByLabel("DataFlowValidation")
	require.NoError(t, err)
	assert.Len(t, validations, 1)

	deleteResp := complianceRequest(t, ts.URL, http.MethodDelete, "/api/v2/compliance/gdpr/data-flows/"+flowID, "")
	require.Equal(t, http.StatusOK, deleteResp.StatusCode)
	deleteBody := decodeBody(t, deleteResp)
	assert.Equal(t, "deleted", deleteBody["status"])

	missingResp := complianceRequest(t, ts.URL, http.MethodGet, "/api/v2/compliance/gdpr/data-flows/"+flowID, "")
	require.Equal(t, http.StatusNotFound, missingResp.StatusCode)
}

func TestComplianceDataFlowErrors(t *testing.T) {
	ts, _, _ := setupComplianceTestServer(t)

	createResp := complianceRequest(t, ts.URL, http.MethodPost, "/api/v2/compliance/gdpr/data-flows", `{bad}`)
	require.Equal(t, http.StatusBadRequest, createResp.StatusCode)
	_ = decodeBody(t, createResp)

	updateResp := complianceRequest(t, ts.URL, http.MethodPut, "/api/v2/compliance/gdpr/data-flows/", `{"name":"broken"}`)
	require.Equal(t, http.StatusBadRequest, updateResp.StatusCode)
	_ = decodeBody(t, updateResp)

	deleteResp := complianceRequest(t, ts.URL, http.MethodDelete, "/api/v2/compliance/gdpr/data-flows/", "")
	require.Equal(t, http.StatusBadRequest, deleteResp.StatusCode)
	_ = decodeBody(t, deleteResp)
}
