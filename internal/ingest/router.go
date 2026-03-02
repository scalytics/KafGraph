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
	"context"
	"log"

	"github.com/scalytics/kafgraph/internal/graph"
)

// Handler processes a single GroupEnvelope and writes to the graph.
type Handler func(ctx context.Context, g *graph.Graph, env *GroupEnvelope, src SourceOffset) error

// Router dispatches envelopes to the appropriate handler by type.
type Router struct {
	handlers map[EnvelopeType]Handler
}

// NewRouter creates a Router with all handlers registered.
func NewRouter() *Router {
	r := &Router{
		handlers: make(map[EnvelopeType]Handler),
	}
	r.handlers[TypeAnnounce] = HandleAnnounce
	r.handlers[TypeRequest] = HandleRequest
	r.handlers[TypeResponse] = HandleResponse
	r.handlers[TypeTaskStatus] = HandleTaskStatus
	r.handlers[TypeSkillRequest] = HandleSkillRequest
	r.handlers[TypeSkillResponse] = HandleSkillResponse
	r.handlers[TypeMemory] = HandleMemory
	r.handlers[TypeTrace] = HandleTrace
	r.handlers[TypeAudit] = HandleAudit
	r.handlers[TypeRoster] = HandleRoster
	r.handlers[TypeOrchestrator] = HandleOrchestrator
	return r
}

// Route dispatches an envelope to the matching handler. Unknown types are logged and skipped.
func (r *Router) Route(ctx context.Context, g *graph.Graph, env *GroupEnvelope, src SourceOffset) error {
	h, ok := r.handlers[env.Type]
	if !ok {
		log.Printf("unknown envelope type %q, skipping", env.Type)
		return nil
	}
	return h(ctx, g, env, src)
}
