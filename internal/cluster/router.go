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
	"fmt"
	"sync"
	"time"

	"github.com/scalytics/kafgraph/internal/query"
)

// QueryRouter fans out queries to all cluster members and merges results.
// It implements QueryExecutor.
type QueryRouter struct {
	localExec    QueryExecutor
	partitionMap *PartitionMap
	self         string
	rpcClient    *RPCClient
	rpcTimeout   time.Duration
}

// NewQueryRouter creates a router that delegates to local and remote executors.
func NewQueryRouter(local QueryExecutor, pm *PartitionMap, self string) *QueryRouter {
	return &QueryRouter{
		localExec:    local,
		partitionMap: pm,
		self:         self,
		rpcClient:    &RPCClient{Timeout: 5 * time.Second},
		rpcTimeout:   5 * time.Second,
	}
}

// Execute implements QueryExecutor. It fans out read queries to all cluster
// nodes and merges results. In single-node mode this is a direct pass-through.
func (r *QueryRouter) Execute(cypher string, params map[string]any) (*query.ResultSet, error) {
	owners := r.partitionMap.UniqueOwners()

	// Single-node fast path: execute locally.
	if len(owners) <= 1 {
		return r.localExec.Execute(cypher, params)
	}

	// Fan-out to all unique owners.
	results := make([]shardResult, len(owners))
	var wg sync.WaitGroup

	for i, owner := range owners {
		if owner.Name == r.self {
			// Local execution (no RPC overhead).
			wg.Go(func() {
				rs, err := r.localExec.Execute(cypher, params)
				results[i] = shardResult{rs: rs, err: err}
			})
		} else {
			// Remote execution via RPC.
			wg.Go(func() {
				addr := fmt.Sprintf("%s:%d", owner.Addr, owner.RPCPort)
				ctx, cancel := context.WithTimeout(context.Background(), r.rpcTimeout)
				defer cancel()
				resp, err := r.rpcClient.Query(ctx, addr, RPCRequest{
					Cypher: cypher,
					Params: params,
				})
				if err != nil {
					results[i] = shardResult{err: err}
					return
				}
				results[i] = shardResult{rs: rpcResponseToResultSet(resp)}
			})
		}
	}
	wg.Wait()

	return mergeResults(results)
}

// shardResult holds the result from a single shard query.
type shardResult struct {
	rs  *query.ResultSet
	err error
}

// mergeResults concatenates rows from all shards and unifies columns.
func mergeResults(results []shardResult) (*query.ResultSet, error) {
	// Collect errors.
	var firstErr error
	var validResults []*query.ResultSet
	for _, r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		if r.rs != nil {
			validResults = append(validResults, r.rs)
		}
	}

	// If all shards failed, return the first error.
	if len(validResults) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return &query.ResultSet{}, nil
	}

	// Use columns from the first valid result.
	merged := &query.ResultSet{
		Columns: validResults[0].Columns,
	}
	for _, rs := range validResults {
		merged.Rows = append(merged.Rows, rs.Rows...)
	}

	return merged, nil
}

// rpcResponseToResultSet converts an RPCResponse to a query.ResultSet.
func rpcResponseToResultSet(resp *RPCResponse) *query.ResultSet {
	rows := make([]query.Row, len(resp.Rows))
	for i, row := range resp.Rows {
		rows[i] = query.Row(row)
	}
	return &query.ResultSet{
		Columns: resp.Columns,
		Rows:    rows,
	}
}
