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

package server

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackStreamEncodeDecodeNil(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, nil))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestPackStreamEncodeDecodeBool(t *testing.T) {
	for _, val := range []bool{true, false} {
		var buf bytes.Buffer
		require.NoError(t, PackStreamEncode(&buf, val))
		v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, val, v)
	}
}

func TestPackStreamEncodeDecodeInt(t *testing.T) {
	cases := []int64{0, 1, -1, 42, -128, 127, 256, -32768, 32767, 100000, -100000}
	for _, val := range cases {
		var buf bytes.Buffer
		require.NoError(t, PackStreamEncode(&buf, val))
		v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, val, v, "for value %d", val)
	}
}

func TestPackStreamEncodeDecodeFloat(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, 3.14))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.InDelta(t, 3.14, v.(float64), 0.001)
}

func TestPackStreamEncodeDecodeString(t *testing.T) {
	cases := []string{"", "hello", "a longer string that exceeds tiny format boundaries"}
	for _, val := range cases {
		var buf bytes.Buffer
		require.NoError(t, PackStreamEncode(&buf, val))
		v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, val, v)
	}
}

func TestPackStreamEncodeDecodeList(t *testing.T) {
	list := []any{int64(1), "hello", true, nil}
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, list))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	result := v.([]any)
	assert.Len(t, result, 4)
	assert.Equal(t, int64(1), result[0])
	assert.Equal(t, "hello", result[1])
	assert.Equal(t, true, result[2])
	assert.Nil(t, result[3])
}

func TestPackStreamEncodeDecodeMap(t *testing.T) {
	m := map[string]any{"name": "alice", "age": int64(30)}
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, m))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	result := v.(map[string]any)
	assert.Equal(t, "alice", result["name"])
	assert.Equal(t, int64(30), result["age"])
}

func TestChunkedWriteRead(t *testing.T) {
	data := []byte("hello bolt world")
	var buf bytes.Buffer
	require.NoError(t, WriteChunked(&buf, data))

	got, err := ReadChunked(&buf)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestChunkedEmpty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteChunked(&buf, nil))

	got, err := ReadChunked(&buf)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestEncodeDecodeMessage(t *testing.T) {
	data, err := EncodeMessage(MsgSUCCESS, map[string]any{"server": "test"})
	require.NoError(t, err)

	// Wrap in chunked transport
	var buf bytes.Buffer
	require.NoError(t, WriteChunked(&buf, data))

	msg, err := DecodeMessage(&buf)
	require.NoError(t, err)
	assert.Equal(t, MsgSUCCESS, msg.Type)
	require.Len(t, msg.Fields, 1)
	meta := msg.Fields[0].(map[string]any)
	assert.Equal(t, "test", meta["server"])
}

func TestSendMessage(t *testing.T) {
	var buf bytes.Buffer
	err := SendMessage(&buf, MsgHELLO, map[string]any{"user_agent": "test/1.0"})
	require.NoError(t, err)

	msg, err := DecodeMessage(&buf)
	require.NoError(t, err)
	assert.Equal(t, MsgHELLO, msg.Type)
}

func TestEncodeDecodeRecord(t *testing.T) {
	record := []any{"alice", int64(30), true}
	var buf bytes.Buffer
	require.NoError(t, SendMessage(&buf, MsgRECORD, record))

	msg, err := DecodeMessage(&buf)
	require.NoError(t, err)
	assert.Equal(t, MsgRECORD, msg.Type)
	require.Len(t, msg.Fields, 1)
	data := msg.Fields[0].([]any)
	assert.Equal(t, "alice", data[0])
	assert.Equal(t, int64(30), data[1])
}

func TestPackStreamEncodeDecodeIntEdgeCases(t *testing.T) {
	cases := []int64{
		-17, -128, -129, -32768, -32769,
		128, 255, 256, 32767, 32768, 65535, 65536,
		2147483647, -2147483648, 2147483648, -2147483649,
	}
	for _, val := range cases {
		var buf bytes.Buffer
		require.NoError(t, PackStreamEncode(&buf, val), "encode %d", val)
		v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err, "decode %d", val)
		assert.Equal(t, val, v, "for value %d", val)
	}
}

func TestPackStreamEncodeDecodeNestedMap(t *testing.T) {
	m := map[string]any{
		"node": map[string]any{
			"id":    "n:1",
			"label": "Agent",
			"properties": map[string]any{
				"name": "alice",
				"age":  int64(30),
			},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, m))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	result := v.(map[string]any)
	node := result["node"].(map[string]any)
	assert.Equal(t, "n:1", node["id"])
	props := node["properties"].(map[string]any)
	assert.Equal(t, "alice", props["name"])
}

func TestPackStreamEncodeUnsupportedType(t *testing.T) {
	var buf bytes.Buffer
	err := PackStreamEncode(&buf, struct{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestPackStreamEncodeLargeString(t *testing.T) {
	// String requiring String8 format (16-255 chars)
	s := string(make([]byte, 200))
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, s))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, s, v)
}

func TestPackStreamEncodeDecodeNegativeTinyInt(t *testing.T) {
	// Test negative tiny int range (-1 to -16)
	for val := int64(-1); val >= -16; val-- {
		var buf bytes.Buffer
		require.NoError(t, PackStreamEncode(&buf, val))
		v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
		require.NoError(t, err)
		assert.Equal(t, val, v, "for value %d", val)
	}
}

func TestPackStreamEncodeDecodeEmptyCollections(t *testing.T) {
	// Empty list
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, []any{}))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, []any{}, v)

	// Empty map
	buf.Reset()
	require.NoError(t, PackStreamEncode(&buf, map[string]any{}))
	v, err = PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, v)
}

func TestPackStreamEncodeInt(t *testing.T) {
	// Test int (not int64)
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, 42))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, int64(42), v)
}

func TestPackStreamLargeString16(t *testing.T) {
	// String requiring String16 format (256+ chars)
	s := make([]byte, 300)
	for i := range s {
		s[i] = 'A'
	}
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, string(s)))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, string(s), v)
}

func TestPackStreamLargeList(t *testing.T) {
	// List requiring List8 format (16+ items)
	list := make([]any, 20)
	for i := range list {
		list[i] = int64(i)
	}
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, list))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	result := v.([]any)
	assert.Len(t, result, 20)
	assert.Equal(t, int64(0), result[0])
	assert.Equal(t, int64(19), result[19])
}

func TestPackStreamLargeMap(t *testing.T) {
	// Map requiring Map8 format (16+ entries)
	m := make(map[string]any, 20)
	for i := range 20 {
		m[fmt.Sprintf("key%02d", i)] = int64(i)
	}
	var buf bytes.Buffer
	require.NoError(t, PackStreamEncode(&buf, m))
	v, err := PackStreamDecode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	result := v.(map[string]any)
	assert.Len(t, result, 20)
}

func TestPackStreamDecodeUnknownMarker(t *testing.T) {
	// Marker 0xEF is not defined in our PackStream subset
	data := []byte{0xEF}
	_, err := PackStreamDecode(bytes.NewReader(data))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown marker")
}

func TestPackStreamStructWithMultipleFields(t *testing.T) {
	// Encode a multi-field message (HELLO with auth map)
	data, err := EncodeMessage(MsgRUN, "MATCH (n) RETURN n", map[string]any{"x": int64(1)})
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, WriteChunked(&buf, data))
	msg, err := DecodeMessage(&buf)
	require.NoError(t, err)
	assert.Equal(t, MsgRUN, msg.Type)
	require.Len(t, msg.Fields, 2)
	assert.Equal(t, "MATCH (n) RETURN n", msg.Fields[0])
}
