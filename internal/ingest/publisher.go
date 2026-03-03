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
	"sync"
)

// Publisher sends messages to an external topic (e.g. Kafka).
type Publisher interface {
	Publish(ctx context.Context, topic string, key string, data []byte) error
}

// PublishedMessage records a message sent via MemoryPublisher.
type PublishedMessage struct {
	Topic string
	Key   string
	Data  []byte
}

// MemoryPublisher is an in-memory Publisher for testing.
type MemoryPublisher struct {
	mu       sync.Mutex
	Messages []PublishedMessage
}

// NewMemoryPublisher creates a new MemoryPublisher.
func NewMemoryPublisher() *MemoryPublisher {
	return &MemoryPublisher{}
}

// Publish stores the message in memory.
func (p *MemoryPublisher) Publish(_ context.Context, topic string, key string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Messages = append(p.Messages, PublishedMessage{
		Topic: topic,
		Key:   key,
		Data:  append([]byte(nil), data...),
	})
	return nil
}

// Len returns the number of published messages.
func (p *MemoryPublisher) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.Messages)
}
