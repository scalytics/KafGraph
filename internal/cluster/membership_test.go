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
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getFreePort asks the OS for an available port.
func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// silentMembership creates a membership with suppressed memberlist logs.
func silentMembership(t *testing.T, name string, pm *PartitionMap) (*Membership, int) {
	t.Helper()
	port := getFreePort(t)
	m, err := NewMembership(MembershipConfig{
		NodeName:  name,
		BindAddr:  "127.0.0.1",
		BindPort:  port,
		RPCPort:   8001,
		BoltPort:  7687,
		HTTPPort:  7474,
		LogOutput: io.Discard,
	}, pm)
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Leave() })
	return m, port
}

func TestMembership_CreateSingle(t *testing.T) {
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	m, _ := silentMembership(t, "test-node-1", pm)

	assert.Equal(t, "test-node-1", m.Self().Name)
	assert.Equal(t, 8001, m.Self().RPCPort)

	members := m.Members()
	assert.Len(t, members, 1)
	assert.Equal(t, "test-node-1", members[0].Name)
}

func TestMembership_JoinTwoNodes(t *testing.T) {
	pm1 := NewPartitionMap(4, &AgentIDPartitioner{})
	m1, port1 := silentMembership(t, "node-1", pm1)

	pm2 := NewPartitionMap(4, &AgentIDPartitioner{})
	m2, _ := silentMembership(t, "node-2", pm2)

	err := m2.Join([]string{fmt.Sprintf("127.0.0.1:%d", port1)})
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	assert.Len(t, m1.Members(), 2)
	assert.Len(t, m2.Members(), 2)
}

func TestMembership_MetadataRoundTrip(t *testing.T) {
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	port := getFreePort(t)
	m, err := NewMembership(MembershipConfig{
		NodeName:  "meta-node",
		BindAddr:  "127.0.0.1",
		BindPort:  port,
		RPCPort:   9001,
		BoltPort:  9002,
		HTTPPort:  9003,
		LogOutput: io.Discard,
	}, pm)
	require.NoError(t, err)
	defer m.Leave()

	members := m.Members()
	require.Len(t, members, 1)
	assert.Equal(t, 9001, members[0].RPCPort)
	assert.Equal(t, 9002, members[0].BoltPort)
	assert.Equal(t, 9003, members[0].HTTPPort)
}

func TestMembership_RebalanceOnJoin(t *testing.T) {
	pm1 := NewPartitionMap(4, &AgentIDPartitioner{})
	m1, port1 := silentMembership(t, "node-a", pm1)
	_ = m1

	assert.Len(t, pm1.LocalPartitions("node-a"), 4)

	pm2 := NewPartitionMap(4, &AgentIDPartitioner{})
	m2, _ := silentMembership(t, "node-b", pm2)

	err := m2.Join([]string{fmt.Sprintf("127.0.0.1:%d", port1)})
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)
	assert.Len(t, pm1.LocalPartitions("node-a"), 2)
	assert.Len(t, pm1.LocalPartitions("node-b"), 2)
}

func TestMembership_OnJoinCallback(t *testing.T) {
	pm1 := NewPartitionMap(4, &AgentIDPartitioner{})
	m1, port1 := silentMembership(t, "callback-a", pm1)

	var joinCount atomic.Int32
	m1.OnJoin(func(NodeInfo) {
		joinCount.Add(1)
	})

	pm2 := NewPartitionMap(4, &AgentIDPartitioner{})
	m2, _ := silentMembership(t, "callback-b", pm2)

	err := m2.Join([]string{fmt.Sprintf("127.0.0.1:%d", port1)})
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)
	assert.Greater(t, joinCount.Load(), int32(0))
}

func TestMembership_JoinEmptySeeds(t *testing.T) {
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	m, _ := silentMembership(t, "solo", pm)

	err := m.Join(nil)
	assert.NoError(t, err, "join with no seeds should succeed")
}

func TestMembership_LeaveAndRejoin(t *testing.T) {
	pm := NewPartitionMap(4, &AgentIDPartitioner{})
	port := getFreePort(t)
	m, err := NewMembership(MembershipConfig{
		NodeName:  "leaver",
		BindAddr:  "127.0.0.1",
		BindPort:  port,
		RPCPort:   8001,
		LogOutput: io.Discard,
	}, pm)
	require.NoError(t, err)

	assert.Len(t, m.Members(), 1)
	err = m.Leave()
	assert.NoError(t, err)
}

func TestNodeMeta_MarshalUnmarshal(t *testing.T) {
	meta := nodeMeta{RPCPort: 8001, BoltPort: 7687, HTTPPort: 7474}
	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var decoded nodeMeta
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, meta, decoded)
}
