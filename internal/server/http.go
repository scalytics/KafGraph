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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/scalytics/kafgraph/internal/brain"
	"github.com/scalytics/kafgraph/internal/config"
	"github.com/scalytics/kafgraph/internal/graph"
	"github.com/scalytics/kafgraph/internal/query"
)

// HTTPServer serves the KafGraph REST API and health endpoints.
type HTTPServer struct {
	server *http.Server
	graph  *graph.Graph
}

// ServerOption configures optional HTTPServer dependencies.
type ServerOption func(*serverOpts)

type serverOpts struct {
	exec  *query.Executor
	brain *brain.Service
}

// WithExecutor sets the query executor.
func WithExecutor(exec *query.Executor) ServerOption {
	return func(o *serverOpts) { o.exec = exec }
}

// WithBrain sets the brain tool service.
func WithBrain(b *brain.Service) ServerOption {
	return func(o *serverOpts) { o.brain = b }
}

// NewHTTPServer creates an HTTPServer with all routes registered.
// Accepts optional *query.Executor for backward compatibility, plus ServerOption.
func NewHTTPServer(addr string, g *graph.Graph, args ...any) *HTTPServer {
	opts := &serverOpts{}
	for _, arg := range args {
		switch v := arg.(type) {
		case *query.Executor:
			opts.exec = v
		case ServerOption:
			v(opts)
		}
	}

	mux := http.NewServeMux()
	s := &HTTPServer{
		server: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		graph: g,
	}

	// Health and info endpoints
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /version", s.handleVersion)

	// Register CRUD + query + brain tool routes
	registerRoutes(mux, g, opts.exec, opts.brain)

	return s
}

// Serve starts the HTTP server. It blocks until the server is shut down.
func (s *HTTPServer) Serve() error {
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

// Handler returns the HTTP handler for use with httptest.NewServer.
func (s *HTTPServer) Handler() http.Handler {
	return s.server.Handler
}

// Shutdown gracefully shuts down the HTTP server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *HTTPServer) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if s.graph == nil {
		writeError(w, http.StatusServiceUnavailable, "graph not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": config.Version,
		"commit":  config.GitCommit,
		"built":   config.BuildDate,
	})
}

// writeJSON encodes data as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// readJSON decodes the request body into dst.
func readJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(dst)
}
