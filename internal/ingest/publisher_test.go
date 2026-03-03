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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryPublisherPublish(t *testing.T) {
	pub := NewMemoryPublisher()
	err := pub.Publish(context.Background(), "topic-1", "key-1", []byte("data-1"))
	require.NoError(t, err)

	assert.Equal(t, 1, pub.Len())
	assert.Equal(t, "topic-1", pub.Messages[0].Topic)
	assert.Equal(t, "key-1", pub.Messages[0].Key)
	assert.Equal(t, []byte("data-1"), pub.Messages[0].Data)
}

func TestMemoryPublisherMultipleMessages(t *testing.T) {
	pub := NewMemoryPublisher()
	pub.Publish(context.Background(), "t1", "k1", []byte("d1"))
	pub.Publish(context.Background(), "t2", "k2", []byte("d2"))
	pub.Publish(context.Background(), "t3", "k3", []byte("d3"))
	assert.Equal(t, 3, pub.Len())
}

func TestMemoryPublisherConcurrent(t *testing.T) {
	pub := NewMemoryPublisher()
	var wg sync.WaitGroup
	n := 100
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			pub.Publish(context.Background(), "topic", "key", []byte("data"))
			_ = i
		}(i)
	}
	wg.Wait()
	assert.Equal(t, n, pub.Len())
}

func TestMemoryPublisherDataIsolation(t *testing.T) {
	pub := NewMemoryPublisher()
	data := []byte("original")
	pub.Publish(context.Background(), "t", "k", data)

	// Mutating the original slice should not affect the stored message
	data[0] = 'X'
	assert.Equal(t, byte('o'), pub.Messages[0].Data[0])
}
