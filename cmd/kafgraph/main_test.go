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

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunStartsAndStops(t *testing.T) {
	// Use a temp dir for storage to avoid polluting the working dir
	dir := t.TempDir()
	os.Setenv("KAFGRAPH_DATA_DIR", dir)
	os.Setenv("KAFGRAPH_PORT", "0")
	os.Setenv("KAFGRAPH_BOLT_PORT", "0")
	defer os.Unsetenv("KAFGRAPH_DATA_DIR")
	defer os.Unsetenv("KAFGRAPH_PORT")
	defer os.Unsetenv("KAFGRAPH_BOLT_PORT")

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx)
	}()

	// Give the server a moment to start, then cancel
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not return within 5 seconds")
	}
}
