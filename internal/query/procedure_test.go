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

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcedureRegisterAndCall(t *testing.T) {
	reg := NewProcedureRegistry()
	reg.Register("test.proc", func(args []any) (*ResultSet, error) {
		return &ResultSet{
			Columns: []string{"value"},
			Rows:    []Row{{"value": args[0]}},
		}, nil
	})

	rs, err := reg.Call("test.proc", []any{"hello"})
	require.NoError(t, err)
	assert.Len(t, rs.Rows, 1)
	assert.Equal(t, "hello", rs.Rows[0]["value"])
}

func TestProcedureUnknown(t *testing.T) {
	reg := NewProcedureRegistry()
	_, err := reg.Call("unknown.proc", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown procedure")
}

func TestProcedureMultipleArgs(t *testing.T) {
	reg := NewProcedureRegistry()
	reg.Register("test.sum", func(args []any) (*ResultSet, error) {
		a := args[0].(int64)
		b := args[1].(int64)
		return &ResultSet{
			Columns: []string{"result"},
			Rows:    []Row{{"result": a + b}},
		}, nil
	})

	rs, err := reg.Call("test.sum", []any{int64(3), int64(4)})
	require.NoError(t, err)
	assert.Equal(t, int64(7), rs.Rows[0]["result"])
}

func TestProcedureOverwrite(t *testing.T) {
	reg := NewProcedureRegistry()
	reg.Register("test.proc", func(_ []any) (*ResultSet, error) {
		return &ResultSet{Columns: []string{"v"}, Rows: []Row{{"v": "old"}}}, nil
	})
	reg.Register("test.proc", func(_ []any) (*ResultSet, error) {
		return &ResultSet{Columns: []string{"v"}, Rows: []Row{{"v": "new"}}}, nil
	})

	rs, err := reg.Call("test.proc", nil)
	require.NoError(t, err)
	assert.Equal(t, "new", rs.Rows[0]["v"])
}
