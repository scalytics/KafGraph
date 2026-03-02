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
	"time"

	"github.com/scalytics/kafgraph/internal/graph"
)

// ProcessorConfig holds configuration for the ingest processor.
type ProcessorConfig struct {
	PollInterval time.Duration
	BatchSize    int
	Namespace    string
}

// Processor reads records from a SegmentReader, parses envelopes, routes
// them through handlers, and checkpoints progress.
type Processor struct {
	reader     SegmentReader
	router     *Router
	checkpoint *CheckpointStore
	graph      *graph.Graph
	config     ProcessorConfig
}

// NewProcessor creates a new Processor.
func NewProcessor(reader SegmentReader, g *graph.Graph, cfg ProcessorConfig) *Processor {
	return &Processor{
		reader:     reader,
		router:     NewRouter(),
		checkpoint: NewCheckpointStore(g, cfg.Namespace),
		graph:      g,
		config:     cfg,
	}
}

// Run starts the poll loop. It blocks until ctx is canceled.
func (p *Processor) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				log.Printf("ingest poll error: %v", err)
			}
		}
	}
}

func (p *Processor) poll(ctx context.Context) error {
	tps, err := p.reader.ListTopicPartitions(ctx)
	if err != nil {
		return err
	}

	for _, tp := range tps {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		afterOffset, err := p.checkpoint.Load(tp.Topic, tp.Partition)
		if err != nil {
			log.Printf("checkpoint load error for %s:%d: %v", tp.Topic, tp.Partition, err)
			continue
		}

		records, err := p.reader.ReadRecords(ctx, tp.Topic, tp.Partition, afterOffset, p.config.BatchSize)
		if err != nil {
			log.Printf("read error for %s:%d: %v", tp.Topic, tp.Partition, err)
			continue
		}

		for _, rec := range records {
			if err := p.ProcessRecord(ctx, rec); err != nil {
				log.Printf("record error at %s:%d:%d: %v",
					rec.Source.Topic, rec.Source.Partition, rec.Source.Offset, err)
				// Skip bad records but still advance the checkpoint
			}

			if err := p.checkpoint.Commit(rec.Source.Topic, rec.Source.Partition, rec.Source.Offset); err != nil {
				return err
			}
		}
	}
	return nil
}

// ProcessRecord parses and routes a single record. Exposed for testing.
func (p *Processor) ProcessRecord(ctx context.Context, rec Record) error {
	env, err := ParseEnvelope(rec.Value)
	if err != nil {
		return err
	}
	return p.router.Route(ctx, p.graph, env, rec.Source)
}
