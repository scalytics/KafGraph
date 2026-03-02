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
}

// NewBoltServer creates a new BoltServer listening on the given address.
func NewBoltServer(addr string) (*BoltServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("bolt listen: %w", err)
	}
	return &BoltServer{listener: ln, addr: addr}, nil
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
	// Future: message framing loop (Phase 2+)
}

// Close stops the BoltServer.
func (s *BoltServer) Close() error {
	return s.listener.Close()
}
