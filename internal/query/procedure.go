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

package query

import "fmt"

// ProcedureFunc is a function that implements a stored procedure.
// It receives arguments and returns a ResultSet.
type ProcedureFunc func(args []any) (*ResultSet, error)

// ProcedureRegistry holds registered stored procedures.
type ProcedureRegistry struct {
	procs map[string]ProcedureFunc
}

// NewProcedureRegistry creates a new empty registry.
func NewProcedureRegistry() *ProcedureRegistry {
	return &ProcedureRegistry{procs: make(map[string]ProcedureFunc)}
}

// Register adds a procedure to the registry.
func (r *ProcedureRegistry) Register(name string, fn ProcedureFunc) {
	r.procs[name] = fn
}

// Call invokes a registered procedure by name.
func (r *ProcedureRegistry) Call(name string, args []any) (*ResultSet, error) {
	fn, ok := r.procs[name]
	if !ok {
		return nil, fmt.Errorf("unknown procedure: %s", name)
	}
	return fn(args)
}
