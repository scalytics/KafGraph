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

func TestCheckpointLoadMissing(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	cs := NewCheckpointStore(g, "test")
	offset, err := cs.Load("topic-1", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(-1), offset)
}

func TestCheckpointCommitAndLoad(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	cs := NewCheckpointStore(g, "test")
	err := cs.Commit("topic-1", 0, 42)
	require.NoError(t, err)

	offset, err := cs.Load("topic-1", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(42), offset)
}

func TestCheckpointOverwrite(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	cs := NewCheckpointStore(g, "test")
	require.NoError(t, cs.Commit("topic-1", 0, 10))
	require.NoError(t, cs.Commit("topic-1", 0, 20))

	offset, err := cs.Load("topic-1", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(20), offset)
}

func TestCheckpointNamespaceIsolation(t *testing.T) {
	g := newTestGraph()
	defer g.Close()

	cs1 := NewCheckpointStore(g, "ns1")
	cs2 := NewCheckpointStore(g, "ns2")

	require.NoError(t, cs1.Commit("topic-1", 0, 100))
	require.NoError(t, cs2.Commit("topic-1", 0, 200))

	o1, err := cs1.Load("topic-1", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(100), o1)

	o2, err := cs2.Load("topic-1", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(200), o2)
}
