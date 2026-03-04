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

import "github.com/scalytics/kafgraph/internal/query"

// NodeInfo describes a cluster member.
type NodeInfo struct {
	Name     string `json:"name"`     // unique node name
	Addr     string `json:"addr"`     // reachable IP/host
	RPCPort  int    `json:"rpcPort"`  // internal RPC port
	BoltPort int    `json:"boltPort"` // external Bolt port
	HTTPPort int    `json:"httpPort"` // external HTTP port
}

// PartitionStrategy assigns an agent ID to a partition number.
type PartitionStrategy interface {
	Partition(agentID string, numPartitions int) int
}

// QueryExecutor abstracts query execution (local or distributed).
// Both *query.Executor and *QueryRouter implement this interface.
type QueryExecutor interface {
	Execute(cypher string, params map[string]any) (*query.ResultSet, error)
}
