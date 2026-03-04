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

//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/cluster"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/query"
	"github.com/scalytics/kafgraph/internal/search"
	"github.com/scalytics/kafgraph/internal/storage"
)

// testNode holds a single in-process cluster node.
type testNode struct {
	name   string
	store  *storage.BadgerStorage
	graph  *graph.Graph
	exec   *query.Executor
	pm     *cluster.PartitionMap
	mem    *cluster.Membership
	rpcSrv *cluster.RPCServer
	router *cluster.QueryRouter
}

func newTestNode(t *testing.T, name string, gossipPort, rpcPort int) *testNode {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	store, err := storage.NewBadgerStorage(dir)
	require.NoError(t, err)
	g := graph.New(store)
	exec := query.NewExecutor(g, nil, nil)

	pm := cluster.NewPartitionMap(8, &cluster.AgentIDPartitioner{})
	mem, err := cluster.NewMembership(cluster.MembershipConfig{
		NodeName: name,
		BindAddr: "127.0.0.1",
		BindPort: gossipPort,
		RPCPort:  rpcPort,
		BoltPort: 0,
		HTTPPort: 0,
	}, pm)
	require.NoError(t, err)

	rpcAddr := fmt.Sprintf("127.0.0.1:%d", rpcPort)
	rpcSrv, err := cluster.NewRPCServer(rpcAddr, exec)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = rpcSrv.Close()
		_ = mem.Leave()
		_ = g.Close()
		_ = store.Close()
	})
	go func() { _ = rpcSrv.Serve(ctx) }()

	router := cluster.NewQueryRouter(exec, pm, name)

	return &testNode{
		name:   name,
		store:  store,
		graph:  g,
		exec:   exec,
		pm:     pm,
		mem:    mem,
		rpcSrv: rpcSrv,
		router: router,
	}
}

func TestE2ECluster_ThreeNodeJoin(t *testing.T) {
	// Create 3 nodes with unique ports.
	n1 := newTestNode(t, "node-1", 30000, 30001)
	n2 := newTestNode(t, "node-2", 30010, 30011)
	n3 := newTestNode(t, "node-3", 30020, 30021)

	// Join nodes 2 and 3 to node 1.
	require.NoError(t, n2.mem.Join([]string{"127.0.0.1:30000"}))
	require.NoError(t, n3.mem.Join([]string{"127.0.0.1:30000"}))

	// Wait for gossip to propagate.
	time.Sleep(500 * time.Millisecond)

	// All nodes should see 3 members.
	assert.Len(t, n1.mem.Members(), 3)
	assert.Len(t, n2.mem.Members(), 3)
	assert.Len(t, n3.mem.Members(), 3)

	// Partitions should be distributed across 3 nodes.
	p1 := n1.pm.LocalPartitions("node-1")
	p2 := n1.pm.LocalPartitions("node-2")
	p3 := n1.pm.LocalPartitions("node-3")
	assert.Greater(t, len(p1), 0, "node-1 should own partitions")
	assert.Greater(t, len(p2), 0, "node-2 should own partitions")
	assert.Greater(t, len(p3), 0, "node-3 should own partitions")
	assert.Equal(t, 8, len(p1)+len(p2)+len(p3), "all partitions assigned")
}

func TestE2ECluster_CrossNodeQuery(t *testing.T) {
	// Create 2 nodes.
	n1 := newTestNode(t, "node-a", 31000, 31001)
	n2 := newTestNode(t, "node-b", 31010, 31011)

	require.NoError(t, n2.mem.Join([]string{"127.0.0.1:31000"}))
	time.Sleep(500 * time.Millisecond)

	// Insert data into node 1.
	_, err := n1.graph.CreateNode("Agent", graph.Properties{"name": "alice"})
	require.NoError(t, err)

	// Insert data into node 2.
	_, err = n2.graph.CreateNode("Agent", graph.Properties{"name": "bob"})
	require.NoError(t, err)

	// Query via node 1's router — should get results from both nodes.
	rs, err := n1.router.Execute("MATCH (n:Agent) RETURN n.name", nil)
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 2, "should see agents from both nodes")

	names := make(map[any]bool)
	for _, row := range rs.Rows {
		names[row["n.name"]] = true
	}
	assert.True(t, names["alice"], "should see alice")
	assert.True(t, names["bob"], "should see bob")
}

func TestE2ECluster_NodeLeave(t *testing.T) {
	n1 := newTestNode(t, "leave-1", 32000, 32001)
	n2 := newTestNode(t, "leave-2", 32010, 32011)

	require.NoError(t, n2.mem.Join([]string{"127.0.0.1:32000"}))
	time.Sleep(500 * time.Millisecond)
	assert.Len(t, n1.mem.Members(), 2)

	// Node 2 leaves.
	require.NoError(t, n2.mem.Leave())
	time.Sleep(500 * time.Millisecond)

	// Node 1 should see only itself.
	assert.Len(t, n1.mem.Members(), 1)
	// All partitions should be owned by node 1.
	assert.Len(t, n1.pm.LocalPartitions("leave-1"), 8)
}

// Ensure search.NewBleveSearcher and search.NewBruteForceVectorSearcher
// are imported correctly (they are used in the main binary).
var _ search.FullTextSearcher = (*search.BleveSearcher)(nil)
