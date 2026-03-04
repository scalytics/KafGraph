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
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/query"
)

// rpcPort extracts the port number from an RPC server address.
func rpcPort(t *testing.T, addr string) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return port
}

func TestRouter_SingleNodeLocalOnly(t *testing.T) {
	exec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"name"},
			Rows:    []query.Row{{"name": "alice"}},
		},
	}
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "self", Addr: "127.0.0.1", RPCPort: 7948},
	})

	router := NewQueryRouter(exec, pm, "self")
	rs, err := router.Execute("MATCH (n) RETURN n.name", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, rs.Columns)
	assert.Len(t, rs.Rows, 1)
	assert.Equal(t, "alice", rs.Rows[0]["name"])
}

func TestRouter_NoOwners(t *testing.T) {
	exec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"n"},
			Rows:    []query.Row{},
		},
	}
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	// No rebalance — no owners.

	router := NewQueryRouter(exec, pm, "self")
	rs, err := router.Execute("MATCH (n) RETURN n", nil)
	require.NoError(t, err)
	assert.Empty(t, rs.Rows)
}

func TestRouter_FanOutWithRPC(t *testing.T) {
	// Local executor returns one row.
	localExec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"name"},
			Rows:    []query.Row{{"name": "local"}},
		},
	}

	// Remote executor (RPC server) returns one row.
	remoteExec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"name"},
			Rows:    []query.Row{{"name": "remote"}},
		},
	}
	srv := startRPCServer(t, remoteExec)

	port := rpcPort(t, srv.Addr())

	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "self", Addr: "127.0.0.1", RPCPort: 7948},
		{Name: "remote", Addr: "127.0.0.1", RPCPort: port},
	})

	router := NewQueryRouter(localExec, pm, "self")
	rs, err := router.Execute("MATCH (n) RETURN n.name", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, rs.Columns)
	assert.Len(t, rs.Rows, 2)

	names := make(map[string]bool)
	for _, row := range rs.Rows {
		names[row["name"].(string)] = true
	}
	assert.True(t, names["local"])
	assert.True(t, names["remote"])
}

func TestRouter_MergeMultipleShards(t *testing.T) {
	localExec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"id"},
			Rows:    []query.Row{{"id": "1"}, {"id": "2"}},
		},
	}
	remoteExec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"id"},
			Rows:    []query.Row{{"id": "3"}, {"id": "4"}},
		},
	}
	srv := startRPCServer(t, remoteExec)
	port := rpcPort(t, srv.Addr())

	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "node-a", Addr: "127.0.0.1", RPCPort: 7948},
		{Name: "node-b", Addr: "127.0.0.1", RPCPort: port},
	})

	router := NewQueryRouter(localExec, pm, "node-a")
	rs, err := router.Execute("MATCH (n) RETURN n.id", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 4)
}

func TestRouter_RemoteError_PartialResults(t *testing.T) {
	localExec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"name"},
			Rows:    []query.Row{{"name": "local"}},
		},
	}

	// Point to a dead address so RPC fails.
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "self", Addr: "127.0.0.1", RPCPort: 7948},
		{Name: "dead", Addr: "127.0.0.1", RPCPort: 1},
	})

	router := NewQueryRouter(localExec, pm, "self")
	router.rpcTimeout = 200 * time.Millisecond

	// Should still return local results even though remote failed.
	rs, err := router.Execute("MATCH (n) RETURN n.name", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
	assert.Equal(t, "local", rs.Rows[0]["name"])
}

func TestRouter_AllShardsError(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("local error")}

	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "self", Addr: "127.0.0.1", RPCPort: 7948},
	})

	router := NewQueryRouter(exec, pm, "self")
	_, err := router.Execute("MATCH (n) RETURN n", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local error")
}

func TestRouter_EmptyResults(t *testing.T) {
	exec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"n"},
			Rows:    []query.Row{},
		},
	}
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "self", Addr: "127.0.0.1", RPCPort: 7948},
	})

	router := NewQueryRouter(exec, pm, "self")
	rs, err := router.Execute("MATCH (n:Nothing) RETURN n", nil)
	require.NoError(t, err)
	assert.Empty(t, rs.Rows)
}

func TestRouter_ImplementsQueryExecutor(t *testing.T) {
	exec := &mockExecutor{}
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	router := NewQueryRouter(exec, pm, "self")

	// Verify it satisfies the QueryExecutor interface.
	var _ QueryExecutor = router
}

func TestRouter_WithParams(t *testing.T) {
	exec := &mockExecutor{
		result: &query.ResultSet{
			Columns: []string{"name"},
			Rows:    []query.Row{{"name": "alice"}},
		},
	}
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	pm.Rebalance([]NodeInfo{
		{Name: "self", Addr: "127.0.0.1", RPCPort: 7948},
	})

	router := NewQueryRouter(exec, pm, "self")
	rs, err := router.Execute("MATCH (n {name: $name}) RETURN n.name", map[string]any{"name": "alice"})
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
}

func TestMergeResults_AllEmpty(t *testing.T) {
	results := []shardResult{
		{rs: &query.ResultSet{Columns: []string{"n"}, Rows: []query.Row{}}},
		{rs: &query.ResultSet{Columns: []string{"n"}, Rows: []query.Row{}}},
	}
	rs, err := mergeResults(results)
	require.NoError(t, err)
	assert.Equal(t, []string{"n"}, rs.Columns)
	assert.Empty(t, rs.Rows)
}
