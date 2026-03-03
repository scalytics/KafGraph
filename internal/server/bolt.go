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

// Package server implements the Bolt v4 protocol handshake and HTTP API.
package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/scalytics/kafgraph/internal/query"
)

const (
	// BoltMagic is the Bolt protocol magic preamble (0x6060B017).
	BoltMagic uint32 = 0x6060B017

	// BoltVersion4_4 represents Bolt protocol version 4.4.
	BoltVersion4_4 uint32 = 0x00000404
)

// BoltServer handles Bolt v4 protocol connections.
type BoltServer struct {
	listener net.Listener
	addr     string
	exec     *query.Executor
}

// NewBoltServer creates a new BoltServer listening on the given address.
// The executor is optional; pass nil for handshake-only mode.
func NewBoltServer(addr string, exec *query.Executor) (*BoltServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("bolt listen: %w", err)
	}
	return &BoltServer{listener: ln, addr: addr, exec: exec}, nil
}

// Addr returns the address the server is listening on.
func (s *BoltServer) Addr() string {
	return s.listener.Addr().String()
}

// Handshake reads the Bolt protocol handshake from a connection and responds
// with the negotiated version. Returns the negotiated version or an error.
func Handshake(conn io.ReadWriter) (uint32, error) {
	// Read magic preamble (4 bytes)
	var magic uint32
	if err := binary.Read(conn, binary.BigEndian, &magic); err != nil {
		return 0, fmt.Errorf("read magic: %w", err)
	}
	if magic != BoltMagic {
		return 0, fmt.Errorf("invalid bolt magic: 0x%08X", magic)
	}

	// Read 4 proposed versions (4 bytes each = 16 bytes)
	versions := make([]uint32, 4)
	for i := range versions {
		if err := binary.Read(conn, binary.BigEndian, &versions[i]); err != nil {
			return 0, fmt.Errorf("read version %d: %w", i, err)
		}
	}

	// Negotiate: accept the first version we support
	negotiated := uint32(0)
	for _, v := range versions {
		if v == BoltVersion4_4 {
			negotiated = v
			break
		}
	}

	// Respond with negotiated version
	if err := binary.Write(conn, binary.BigEndian, negotiated); err != nil {
		return 0, fmt.Errorf("write version: %w", err)
	}

	return negotiated, nil
}

// Serve accepts connections in a loop until the context is canceled.
func (s *BoltServer) Serve(ctx context.Context) error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil // clean shutdown
			default:
				return fmt.Errorf("bolt accept: %w", err)
			}
		}
		go s.handleConn(conn)
	}
}

func (s *BoltServer) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	version, err := Handshake(conn)
	if err != nil {
		log.Printf("bolt handshake error: %v", err)
		return
	}
	if version == 0 {
		log.Printf("bolt: no supported version negotiated")
		return
	}
	log.Printf("bolt: negotiated version %d.%d", version>>8, version&0xFF)

	if s.exec == nil {
		return // handshake-only mode
	}

	// Full message loop
	s.messageLoop(conn)
}

func (s *BoltServer) messageLoop(conn net.Conn) {
	for {
		msg, err := DecodeMessage(conn)
		if err != nil {
			return // connection closed or error
		}

		switch msg.Type {
		case MsgHELLO:
			// Accept any auth in v1
			meta := map[string]any{"server": "KafGraph/1.0", "connection_id": "bolt-1"}
			if err := SendMessage(conn, MsgSUCCESS, meta); err != nil {
				return
			}

		case MsgRUN:
			if len(msg.Fields) < 1 {
				sendFailure(conn, "missing query")
				continue
			}
			cypher, ok := msg.Fields[0].(string)
			if !ok {
				sendFailure(conn, "query must be string")
				continue
			}
			params := extractParams(msg)
			rs, err := s.exec.Execute(cypher, params)
			if err != nil {
				sendFailure(conn, err.Error())
				continue
			}
			// Send SUCCESS with column metadata
			meta := map[string]any{"fields": toAnySlice(rs.Columns)}
			if err := SendMessage(conn, MsgSUCCESS, meta); err != nil {
				return
			}
			// Wait for PULL, then stream records
			pullMsg, err := DecodeMessage(conn)
			if err != nil {
				return
			}
			if pullMsg.Type == MsgPULL {
				for _, row := range rs.Rows {
					record := make([]any, len(rs.Columns))
					for i, col := range rs.Columns {
						record[i] = row[col]
					}
					if err := SendMessage(conn, MsgRECORD, record); err != nil {
						return
					}
				}
				if err := SendMessage(conn, MsgSUCCESS, map[string]any{}); err != nil {
					return
				}
			}

		case MsgRESET:
			if err := SendMessage(conn, MsgSUCCESS, map[string]any{}); err != nil {
				return
			}

		default:
			sendFailure(conn, fmt.Sprintf("unknown message type: 0x%02X", msg.Type))
		}
	}
}

func sendFailure(conn net.Conn, message string) {
	_ = SendMessage(conn, MsgFAILURE, map[string]any{
		"code":    "Neo.ClientError.Statement.SyntaxError",
		"message": message,
	})
}

func extractParams(msg *BoltMessage) map[string]any {
	if len(msg.Fields) < 2 {
		return nil
	}
	if m, ok := msg.Fields[1].(map[string]any); ok {
		return m
	}
	return nil
}

func toAnySlice(ss []string) []any {
	result := make([]any, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

// Close stops the BoltServer.
func (s *BoltServer) Close() error {
	return s.listener.Close()
}
