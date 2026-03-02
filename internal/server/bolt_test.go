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
	srv, err := NewBoltServer("127.0.0.1:0")
	require.NoError(t, err)
	defer srv.Close()

	assert.NotEmpty(t, srv.Addr())
}

func TestBoltServeAcceptsConnection(t *testing.T) {
	srv, err := NewBoltServer("127.0.0.1:0")
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

// readWriter combines separate reader and writer for testing.
type readWriter struct {
	reader *bytes.Buffer
	writer *bytes.Buffer
}

func (rw *readWriter) Read(p []byte) (int, error)  { return rw.reader.Read(p) }
func (rw *readWriter) Write(p []byte) (int, error) { return rw.writer.Write(p) }
