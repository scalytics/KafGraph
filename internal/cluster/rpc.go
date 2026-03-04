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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// RPCRequest is the wire format for internal queries.
type RPCRequest struct {
	Cypher string         `json:"cypher"`
	Params map[string]any `json:"params,omitempty"`
}

// RPCResponse is the wire format for query results.
type RPCResponse struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Error   string           `json:"error,omitempty"`
}

// RPCServer listens for internal RPC calls and executes queries locally.
type RPCServer struct {
	listener net.Listener
	exec     QueryExecutor
	wg       sync.WaitGroup
}

// NewRPCServer creates a new RPC server listening on the given address.
func NewRPCServer(addr string, exec QueryExecutor) (*RPCServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("rpc listen: %w", err)
	}
	return &RPCServer{listener: ln, exec: exec}, nil
}

// Addr returns the address the server is listening on.
func (s *RPCServer) Addr() string {
	return s.listener.Addr().String()
}

// Serve accepts connections until the context is canceled.
func (s *RPCServer) Serve(ctx context.Context) error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				s.wg.Wait()
				return nil
			default:
				return fmt.Errorf("rpc accept: %w", err)
			}
		}
		s.wg.Go(func() {
			s.handleConn(conn)
		})
	}
}

// Close stops the RPC server.
func (s *RPCServer) Close() error {
	err := s.listener.Close()
	s.wg.Wait()
	return err
}

func (s *RPCServer) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	for {
		req, err := readRPCMessage[RPCRequest](conn)
		if err != nil {
			return // connection closed or error
		}

		resp := s.executeRequest(req)
		if err := writeRPCMessage(conn, resp); err != nil {
			return
		}
	}
}

func (s *RPCServer) executeRequest(req *RPCRequest) *RPCResponse {
	rs, err := s.exec.Execute(req.Cypher, req.Params)
	if err != nil {
		return &RPCResponse{Error: err.Error()}
	}
	rows := make([]map[string]any, len(rs.Rows))
	for i, row := range rs.Rows {
		rows[i] = map[string]any(row)
	}
	return &RPCResponse{
		Columns: rs.Columns,
		Rows:    rows,
	}
}

// RPCClient sends queries to remote nodes.
type RPCClient struct {
	Timeout time.Duration
}

// Query sends a query to the given remote address and returns the response.
func (c *RPCClient) Query(ctx context.Context, addr string, req RPCRequest) (*RPCResponse, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("rpc dial %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}

	if err := writeRPCMessage(conn, &req); err != nil {
		return nil, fmt.Errorf("rpc write to %s: %w", addr, err)
	}

	resp, err := readRPCMessage[RPCResponse](conn)
	if err != nil {
		return nil, fmt.Errorf("rpc read from %s: %w", addr, err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("remote error from %s: %s", addr, resp.Error)
	}

	return resp, nil
}

// Wire format: [4-byte big-endian length][JSON payload]

func writeRPCMessage(w io.Writer, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal rpc message: %w", err)
	}
	if len(data) > int(^uint32(0)) {
		return fmt.Errorf("rpc message too large: %d bytes", len(data))
	}
	length := uint32(len(data)) //nolint:gosec // bounds checked above
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("write rpc length: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write rpc payload: %w", err)
	}
	return nil
}

func readRPCMessage[T any](r io.Reader) (*T, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length > 10*1024*1024 { // 10 MB safety limit
		return nil, fmt.Errorf("rpc message too large: %d bytes", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read rpc payload: %w", err)
	}
	var msg T
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal rpc message: %w", err)
	}
	return &msg, nil
}
