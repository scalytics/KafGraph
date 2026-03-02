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
	"sort"
	"sync"
)

// SegmentReader abstracts reading records from segment storage.
// The real implementation reads from S3 via KafScale; MemoryReader
// is provided for testing.
type SegmentReader interface {
	// ReadRecords returns up to maxRecords records from the given topic-partition
	// with offsets strictly greater than afterOffset.
	ReadRecords(ctx context.Context, topic string, partition int32, afterOffset int64, maxRecords int) ([]Record, error)

	// ListTopicPartitions returns all known topic-partitions.
	ListTopicPartitions(ctx context.Context) ([]TopicPartition, error)
}

// MemoryReader is an in-memory SegmentReader for testing.
type MemoryReader struct {
	mu      sync.RWMutex
	records map[string][]Record // key: "topic:partition"
}

// NewMemoryReader creates an empty MemoryReader.
func NewMemoryReader() *MemoryReader {
	return &MemoryReader{
		records: make(map[string][]Record),
	}
}

func tpKey(topic string, partition int32) string {
	return topic + ":" + itoa(partition)
}

func itoa(n int32) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	if n < 0 {
		buf = append(buf, '-')
		n = -n
	}
	// Build digits in reverse
	start := len(buf)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// Reverse digit portion
	for i, j := start, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// AddRecord appends a record to the memory store.
func (m *MemoryReader) AddRecord(topic string, partition int32, offset int64, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := tpKey(topic, partition)
	m.records[key] = append(m.records[key], Record{
		Source: SourceOffset{Topic: topic, Partition: partition, Offset: offset},
		Value:  value,
	})
}

// ReadRecords returns records after the given offset.
func (m *MemoryReader) ReadRecords(_ context.Context, topic string, partition int32, afterOffset int64, maxRecords int) ([]Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := tpKey(topic, partition)
	all := m.records[key]

	// Sort by offset
	sorted := make([]Record, len(all))
	copy(sorted, all)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Source.Offset < sorted[j].Source.Offset
	})

	var result []Record
	for _, r := range sorted {
		if r.Source.Offset > afterOffset {
			result = append(result, r)
			if len(result) >= maxRecords {
				break
			}
		}
	}
	return result, nil
}

// ListTopicPartitions returns all topic-partitions that have records.
func (m *MemoryReader) ListTopicPartitions(_ context.Context) ([]TopicPartition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[string]bool)
	var result []TopicPartition
	for _, recs := range m.records {
		for _, r := range recs {
			key := tpKey(r.Source.Topic, r.Source.Partition)
			if !seen[key] {
				seen[key] = true
				result = append(result, TopicPartition{
					Topic:     r.Source.Topic,
					Partition: r.Source.Partition,
				})
			}
		}
	}
	return result, nil
}
