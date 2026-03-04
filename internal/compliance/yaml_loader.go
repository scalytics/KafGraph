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

package compliance

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadYAMLRules reads YAML rule files from dir (recursively) and registers them.
func (e *Engine) LoadYAMLRules(dir string) error {
	if dir == "" {
		return nil
	}

	// Check if directory exists.
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no rules directory is fine
		}
		return fmt.Errorf("stat rules dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		return e.loadYAMLFile(path)
	})
}

func (e *Engine) loadYAMLFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var def YAMLRuleDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if def.RuleID == "" {
		return fmt.Errorf("rule in %s has no id", path)
	}

	e.RegisterRule(def.ToRule())
	return nil
}

// ParseYAMLRule parses a single YAML rule definition from bytes.
func ParseYAMLRule(data []byte) (*YAMLRuleDef, error) {
	var def YAMLRuleDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	if def.RuleID == "" {
		return nil, fmt.Errorf("rule has no id")
	}
	return &def, nil
}
