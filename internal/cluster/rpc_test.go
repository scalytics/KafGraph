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

package cluster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/query"
)

// mockExecutor is a simple QueryExecutor for testing.
type mockExecutor struct {
	result *query.ResultSet
	err    error
}

func (m *mockExecutor) Execute(cypher string, params map[string]any) (*query.ResultSet, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &query.ResultSet{
		Columns: []string{"cypher"},
		Rows:    []query.Row{{"cypher": cypher}},
	}, nil
}

func startRPCServer(t *testing.T, exec QueryExecutor) *RPCServer {
	t.Helper()
	srv, err := NewRPCServer("127.0.0.1:0", exec)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = srv.Close()
	})
	go func() { _ = srv.Serve(ctx) }()
	return srv
}

func TestRPC_RequestResponseRoundTrip(t *testing.T) {
	exec := &mockExecutor{}
	srv := startRPCServer(t, exec)

	client := &RPCClient{Timeout: 2 * time.Second}
	resp, err := client.Query(context.Background(), srv.Addr(), RPCRequest{
		Cypher: "MATCH (n) RETURN n",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []string{"cypher"}, resp.Columns)
	assert.Len(t, resp.Rows, 1)
	assert.Equal(t, "MATCH (n) RETURN n", resp.Rows[0]["cypher"])
}

func TestRPC_WithParams(t *testing.T) {
	exec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"name"},
			Rows:    []query.Row{{"name": "alice"}},
		},
	}
	srv := startRPCServer(t, exec)

	client := &RPCClient{Timeout: 2 * time.Second}
	resp, err := client.Query(context.Background(), srv.Addr(), RPCRequest{
		Cypher: "MATCH (n:Agent {name: $name}) RETURN n.name",
		Params: map[string]any{"name": "alice"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, resp.Columns)
	assert.Equal(t, "alice", resp.Rows[0]["name"])
}

func TestRPC_ErrorPropagation(t *testing.T) {
	exec := &mockExecutor{
		err: assert.AnError,
	}
	srv := startRPCServer(t, exec)

	client := &RPCClient{Timeout: 2 * time.Second}
	_, err := client.Query(context.Background(), srv.Addr(), RPCRequest{
		Cypher: "INVALID",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote error")
}

func TestRPC_ConnectionRefused(t *testing.T) {
	client := &RPCClient{Timeout: 500 * time.Millisecond}
	_, err := client.Query(context.Background(), "127.0.0.1:1", RPCRequest{
		Cypher: "MATCH (n) RETURN n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rpc dial")
}

func TestRPC_ContextCancellation(t *testing.T) {
	exec := &mockExecutor{}
	srv := startRPCServer(t, exec)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	client := &RPCClient{Timeout: 2 * time.Second}
	_, err := client.Query(ctx, srv.Addr(), RPCRequest{
		Cypher: "MATCH (n) RETURN n",
	})
	require.Error(t, err)
}

func TestRPC_ConcurrentQueries(t *testing.T) {
	exec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"id"},
			Rows:    []query.Row{{"id": "test"}},
		},
	}
	srv := startRPCServer(t, exec)

	const numClients = 10
	var wg sync.WaitGroup
	errors := make(chan error, numClients)

	for range numClients {
		wg.Go(func() {
			client := &RPCClient{Timeout: 5 * time.Second}
			_, err := client.Query(context.Background(), srv.Addr(), RPCRequest{
				Cypher: "MATCH (n) RETURN n",
			})
			if err != nil {
				errors <- err
			}
		})
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent query failed: %v", err)
	}
}

func TestRPC_EmptyResultSet(t *testing.T) {
	exec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"n"},
			Rows:    []query.Row{},
		},
	}
	srv := startRPCServer(t, exec)

	client := &RPCClient{Timeout: 2 * time.Second}
	resp, err := client.Query(context.Background(), srv.Addr(), RPCRequest{
		Cypher: "MATCH (n:Nothing) RETURN n",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"n"}, resp.Columns)
	assert.Empty(t, resp.Rows)
}

func TestRPC_ServerClose(t *testing.T) {
	exec := &mockExecutor{}
	srv, err := NewRPCServer("127.0.0.1:0", exec)
	require.NoError(t, err)

	addr := srv.Addr()
	err = srv.Close()
	require.NoError(t, err)

	client := &RPCClient{Timeout: 500 * time.Millisecond}
	_, err = client.Query(context.Background(), addr, RPCRequest{
		Cypher: "MATCH (n) RETURN n",
	})
	require.Error(t, err)
}
