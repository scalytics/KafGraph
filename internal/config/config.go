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
	"strings"

	"github.com/spf13/viper"
)

// Build-time variables set via -ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Config holds the application configuration.
type Config struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	BoltPort int    `mapstructure:"bolt_port"`

	DataDir       string `mapstructure:"data_dir"`
	StorageEngine string `mapstructure:"storage_engine"`

	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`

	Kafka KafkaConfig `mapstructure:"kafka"`
	S3    S3Config    `mapstructure:"s3"`
}

// KafkaConfig holds Kafka connection settings.
type KafkaConfig struct {
	Brokers     string `mapstructure:"brokers"`
	GroupID     string `mapstructure:"group_id"`
	TopicPrefix string `mapstructure:"topic_prefix"`
}

// S3Config holds S3/MinIO connection settings.
type S3Config struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

// Load reads configuration from file and environment variables.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("port", 7474)
	v.SetDefault("bolt_port", 7687)
	v.SetDefault("data_dir", "./data")
	v.SetDefault("storage_engine", "badger")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")
	v.SetDefault("kafka.brokers", "localhost:9092")
	v.SetDefault("kafka.group_id", "kafgraph")
	v.SetDefault("kafka.topic_prefix", "group")
	v.SetDefault("s3.endpoint", "localhost:9000")
	v.SetDefault("s3.bucket", "kafgraph")
	v.SetDefault("s3.use_ssl", false)

	// Config file
	v.SetConfigName("kafgraph")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/kafgraph")

	// Environment variables: KAFGRAPH_HOST, KAFGRAPH_KAFKA_BROKERS, etc.
	v.SetEnvPrefix("KAFGRAPH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
