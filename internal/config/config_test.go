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

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, 7474, cfg.Port)
	assert.Equal(t, 7687, cfg.BoltPort)
	assert.Equal(t, "./data", cfg.DataDir)
	assert.Equal(t, "badger", cfg.StorageEngine)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("KAFGRAPH_PORT", "8080")
	os.Setenv("KAFGRAPH_LOG_LEVEL", "debug")
	defer os.Unsetenv("KAFGRAPH_PORT")
	defer os.Unsetenv("KAFGRAPH_LOG_LEVEL")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestVersionVars(t *testing.T) {
	assert.Equal(t, "dev", Version)
	assert.Equal(t, "unknown", GitCommit)
	assert.Equal(t, "unknown", BuildDate)
}
