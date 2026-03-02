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

package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnvelopeValid(t *testing.T) {
	data := []byte(`{
		"Type": "announce",
		"CorrelationID": "corr-1",
		"SenderID": "agent-1",
		"Timestamp": "2026-03-01T12:00:00Z",
		"Payload": {"AgentID":"agent-1","AgentName":"alice","Action":"join","GroupName":"team-1"}
	}`)

	env, err := ParseEnvelope(data)
	require.NoError(t, err)
	assert.Equal(t, TypeAnnounce, env.Type)
	assert.Equal(t, "corr-1", env.CorrelationID)
	assert.Equal(t, "agent-1", env.SenderID)
	assert.False(t, env.Timestamp.IsZero())
	assert.NotEmpty(t, env.Payload)
}

func TestParseEnvelopeAllTypes(t *testing.T) {
	types := []EnvelopeType{
		TypeAnnounce, TypeRequest, TypeResponse, TypeTaskStatus,
		TypeSkillRequest, TypeSkillResponse, TypeMemory,
		TypeTrace, TypeAudit, TypeRoster, TypeOrchestrator,
	}
	for _, typ := range types {
		data := []byte(`{"Type":"` + string(typ) + `","CorrelationID":"c","SenderID":"s","Timestamp":"2026-03-01T12:00:00Z","Payload":{}}`)
		env, err := ParseEnvelope(data)
		require.NoError(t, err, "type: %s", typ)
		assert.Equal(t, typ, env.Type)
	}
}

func TestParseEnvelopeInvalidJSON(t *testing.T) {
	_, err := ParseEnvelope([]byte(`not json`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse envelope")
}

func TestParseEnvelopeMissingType(t *testing.T) {
	data := []byte(`{"CorrelationID":"c","SenderID":"s","Timestamp":"2026-03-01T12:00:00Z","Payload":{}}`)
	_, err := ParseEnvelope(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing Type")
}

func TestParseEnvelopeEmptyPayload(t *testing.T) {
	data := []byte(`{"Type":"announce","CorrelationID":"c","SenderID":"s","Timestamp":"2026-03-01T12:00:00Z"}`)
	env, err := ParseEnvelope(data)
	require.NoError(t, err)
	assert.Equal(t, TypeAnnounce, env.Type)
	assert.Nil(t, env.Payload)
}

func FuzzParseEnvelope(f *testing.F) {
	f.Add([]byte(`{"Type":"announce","CorrelationID":"c","SenderID":"s","Timestamp":"2026-03-01T12:00:00Z","Payload":{}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(``))

	f.Fuzz(func(_ *testing.T, data []byte) {
		// Must not panic
		ParseEnvelope(data) //nolint:errcheck
	})
}
