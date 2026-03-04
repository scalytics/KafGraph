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
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/query"
)

func TestHandshakeSuccess(t *testing.T) {
	// Build a valid handshake request: magic + 4 versions
	var req bytes.Buffer
	binary.Write(&req, binary.BigEndian, BoltMagic)
	binary.Write(&req, binary.BigEndian, BoltVersion4_4) // version 1
	binary.Write(&req, binary.BigEndian, uint32(0))      // version 2
	binary.Write(&req, binary.BigEndian, uint32(0))      // version 3
	binary.Write(&req, binary.BigEndian, uint32(0))      // version 4

	var resp bytes.Buffer
	rw := &readWriter{reader: &req, writer: &resp}

	version, err := Handshake(rw)
	require.NoError(t, err)
	assert.Equal(t, BoltVersion4_4, version)

	// Verify response
	var negotiated uint32
	binary.Read(&resp, binary.BigEndian, &negotiated)
	assert.Equal(t, BoltVersion4_4, negotiated)
}

func TestHandshakeInvalidMagic(t *testing.T) {
	var req bytes.Buffer
	binary.Write(&req, binary.BigEndian, uint32(0xDEADBEEF))

	rw := &readWriter{reader: &req, writer: &bytes.Buffer{}}
	_, err := Handshake(rw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bolt magic")
}

func TestHandshakeNoSupportedVersion(t *testing.T) {
	var req bytes.Buffer
	binary.Write(&req, binary.BigEndian, BoltMagic)
	binary.Write(&req, binary.BigEndian, uint32(0x00000300)) // v3.0
	binary.Write(&req, binary.BigEndian, uint32(0x00000200)) // v2.0
	binary.Write(&req, binary.BigEndian, uint32(0))
	binary.Write(&req, binary.BigEndian, uint32(0))

	var resp bytes.Buffer
	rw := &readWriter{reader: &req, writer: &resp}

	version, err := Handshake(rw)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), version) // no version negotiated

	var negotiated uint32
	binary.Read(&resp, binary.BigEndian, &negotiated)
	assert.Equal(t, uint32(0), negotiated)
}

func TestNewBoltServer(t *testing.T) {
	srv, err := NewBoltServer("127.0.0.1:0", nil)
	require.NoError(t, err)
	defer srv.Close()

	assert.NotEmpty(t, srv.Addr())
}

func TestBoltServeAcceptsConnection(t *testing.T) {
	srv, err := NewBoltServer("127.0.0.1:0", nil)
	require.NoError(t, err)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx) }()

	// Dial and perform handshake
	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer conn.Close()

	// Send handshake: magic + 4 versions
	binary.Write(conn, binary.BigEndian, BoltMagic)
	binary.Write(conn, binary.BigEndian, BoltVersion4_4)
	binary.Write(conn, binary.BigEndian, uint32(0))
	binary.Write(conn, binary.BigEndian, uint32(0))
	binary.Write(conn, binary.BigEndian, uint32(0))

	// Read negotiated version
	var negotiated uint32
	err = binary.Read(conn, binary.BigEndian, &negotiated)
	require.NoError(t, err)
	assert.Equal(t, BoltVersion4_4, negotiated)

	// Cancel to stop the server
	cancel()
	srv.Close()
}

func TestBoltMessageLoop(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	defer g.Close()

	g.CreateNode("Agent", graph.Properties{"name": "alice"})
	g.CreateNode("Agent", graph.Properties{"name": "bob"})

	exec := query.NewExecutor(g, nil, nil)

	srv, err := NewBoltServer("127.0.0.1:0", exec)
	require.NoError(t, err)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx) //nolint:errcheck

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer conn.Close()

	// Handshake
	binary.Write(conn, binary.BigEndian, BoltMagic)      //nolint:errcheck
	binary.Write(conn, binary.BigEndian, BoltVersion4_4) //nolint:errcheck
	binary.Write(conn, binary.BigEndian, uint32(0))      //nolint:errcheck
	binary.Write(conn, binary.BigEndian, uint32(0))      //nolint:errcheck
	binary.Write(conn, binary.BigEndian, uint32(0))      //nolint:errcheck

	var negotiated uint32
	require.NoError(t, binary.Read(conn, binary.BigEndian, &negotiated))
	require.Equal(t, BoltVersion4_4, negotiated)

	// Send HELLO
	require.NoError(t, SendMessage(conn, MsgHELLO, map[string]any{"user_agent": "test/1.0"}))
	helloResp, err := DecodeMessage(conn)
	require.NoError(t, err)
	assert.Equal(t, MsgSUCCESS, helloResp.Type)

	// Send RUN
	require.NoError(t, SendMessage(conn, MsgRUN, "MATCH (n:Agent) RETURN n", map[string]any{}))
	runResp, err := DecodeMessage(conn)
	require.NoError(t, err)
	assert.Equal(t, MsgSUCCESS, runResp.Type)

	// Send PULL
	require.NoError(t, SendMessage(conn, MsgPULL, map[string]any{"n": int64(-1)}))

	// Read RECORD messages
	var records int
	for {
		msg, err := DecodeMessage(conn)
		require.NoError(t, err)
		if msg.Type == MsgSUCCESS {
			break
		}
		assert.Equal(t, MsgRECORD, msg.Type)
		records++
	}
	assert.Equal(t, 2, records)

	// Send RESET
	require.NoError(t, SendMessage(conn, MsgRESET))
	resetResp, err := DecodeMessage(conn)
	require.NoError(t, err)
	assert.Equal(t, MsgSUCCESS, resetResp.Type)
}

func TestBoltMessageLoopInvalidQuery(t *testing.T) {
	store := newBadgerTestStorage(t)
	g := graph.New(store)
	defer g.Close()

	exec := query.NewExecutor(g, nil, nil)

	srv, err := NewBoltServer("127.0.0.1:0", exec)
	require.NoError(t, err)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx) //nolint:errcheck

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	defer conn.Close()

	// Handshake
	binary.Write(conn, binary.BigEndian, BoltMagic)      //nolint:errcheck
	binary.Write(conn, binary.BigEndian, BoltVersion4_4) //nolint:errcheck
	binary.Write(conn, binary.BigEndian, uint32(0))      //nolint:errcheck
	binary.Write(conn, binary.BigEndian, uint32(0))      //nolint:errcheck
	binary.Write(conn, binary.BigEndian, uint32(0))      //nolint:errcheck

	var negotiated uint32
	require.NoError(t, binary.Read(conn, binary.BigEndian, &negotiated))

	// HELLO
	require.NoError(t, SendMessage(conn, MsgHELLO, map[string]any{}))
	_, err = DecodeMessage(conn)
	require.NoError(t, err)

	// RUN with invalid query
	require.NoError(t, SendMessage(conn, MsgRUN, "INVALID QUERY", map[string]any{}))
	resp, err := DecodeMessage(conn)
	require.NoError(t, err)
	assert.Equal(t, MsgFAILURE, resp.Type)
}

func TestExtractParams(t *testing.T) {
	msg := &BoltMessage{Type: MsgRUN, Fields: []any{"MATCH (n) RETURN n", map[string]any{"x": int64(1)}}}
	params := extractParams(msg)
	assert.Equal(t, int64(1), params["x"])

	// No params field
	msg2 := &BoltMessage{Type: MsgRUN, Fields: []any{"query"}}
	assert.Nil(t, extractParams(msg2))

	// Wrong type
	msg3 := &BoltMessage{Type: MsgRUN, Fields: []any{"query", "not-a-map"}}
	assert.Nil(t, extractParams(msg3))
}

func TestToAnySlice(t *testing.T) {
	result := toAnySlice([]string{"a", "b", "c"})
	assert.Equal(t, []any{"a", "b", "c"}, result)
	assert.Empty(t, toAnySlice(nil))
}

// readWriter combines separate reader and writer for testing.
type readWriter struct {
	reader *bytes.Buffer
	writer *bytes.Buffer
}

func (rw *readWriter) Read(p []byte) (int, error)  { return rw.reader.Read(p) }
func (rw *readWriter) Write(p []byte) (int, error) { return rw.writer.Write(p) }
