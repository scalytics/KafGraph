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
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Bolt v4 message type markers.
const (
	MsgHELLO   byte = 0x01
	MsgRUN     byte = 0x10
	MsgPULL    byte = 0x3F
	MsgSUCCESS byte = 0x70
	MsgRECORD  byte = 0x71
	MsgFAILURE byte = 0x7F
	MsgRESET   byte = 0x0F
)

// PackStream marker bytes.
const (
	psNil      byte = 0xC0
	psFalse    byte = 0xC2
	psTrue     byte = 0xC3
	psInt8     byte = 0xC8
	psInt16    byte = 0xC9
	psInt32    byte = 0xCA
	psInt64    byte = 0xCB
	psFloat64  byte = 0xC1
	psString8  byte = 0xD0
	psString16 byte = 0xD1
	psString32 byte = 0xD2
	psList8    byte = 0xD4
	psList16   byte = 0xD5
	psMap8     byte = 0xD8
	psMap16    byte = 0xD9
	psStruct8  byte = 0xDC
)

// BoltMessage represents a decoded Bolt protocol message.
type BoltMessage struct {
	Type   byte
	Fields []any
}

// --- PackStream Encoder ---

// PackStreamEncode encodes a value into PackStream format.
func PackStreamEncode(buf *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case nil:
		buf.WriteByte(psNil)
	case bool:
		if val {
			buf.WriteByte(psTrue)
		} else {
			buf.WriteByte(psFalse)
		}
	case int:
		return packInt(buf, int64(val))
	case int64:
		return packInt(buf, val)
	case float64:
		buf.WriteByte(psFloat64)
		return binary.Write(buf, binary.BigEndian, val)
	case string:
		return packString(buf, val)
	case []any:
		return packList(buf, val)
	case map[string]any:
		return packMap(buf, val)
	default:
		return fmt.Errorf("packstream: unsupported type %T", v)
	}
	return nil
}

func packInt(buf *bytes.Buffer, v int64) error {
	if v >= -16 && v <= 127 {
		buf.WriteByte(byte(v)) //nolint:gosec // range checked: [-16, 127]
		return nil
	}
	if v >= math.MinInt8 && v <= math.MaxInt8 {
		buf.WriteByte(psInt8)
		buf.WriteByte(byte(v)) //nolint:gosec // range checked: [MinInt8, MaxInt8]
		return nil
	}
	if v >= math.MinInt16 && v <= math.MaxInt16 {
		buf.WriteByte(psInt16)
		return binary.Write(buf, binary.BigEndian, int16(v))
	}
	if v >= math.MinInt32 && v <= math.MaxInt32 {
		buf.WriteByte(psInt32)
		return binary.Write(buf, binary.BigEndian, int32(v))
	}
	buf.WriteByte(psInt64)
	return binary.Write(buf, binary.BigEndian, v)
}

func packString(buf *bytes.Buffer, s string) error {
	n := len(s)
	if n < 16 {
		buf.WriteByte(0x80 | byte(n)) // tiny string
	} else if n <= 0xFF {
		buf.WriteByte(psString8)
		buf.WriteByte(byte(n))
	} else if n <= 0xFFFF {
		buf.WriteByte(psString16)
		if err := binary.Write(buf, binary.BigEndian, uint16(n)); err != nil {
			return err
		}
	} else {
		buf.WriteByte(psString32)
		if err := binary.Write(buf, binary.BigEndian, uint32(n)); err != nil { //nolint:gosec // n is len(string), fits uint32
			return err
		}
	}
	buf.WriteString(s)
	return nil
}

func packList(buf *bytes.Buffer, list []any) error {
	n := len(list)
	if n < 16 {
		buf.WriteByte(0x90 | byte(n)) // tiny list
	} else if n <= 0xFF {
		buf.WriteByte(psList8)
		buf.WriteByte(byte(n))
	} else {
		buf.WriteByte(psList16)
		if err := binary.Write(buf, binary.BigEndian, uint16(n)); err != nil { //nolint:gosec // n > 0xFF, bounded by practical list size
			return err
		}
	}
	for _, item := range list {
		if err := PackStreamEncode(buf, item); err != nil {
			return err
		}
	}
	return nil
}

func packMap(buf *bytes.Buffer, m map[string]any) error {
	n := len(m)
	if n < 16 {
		buf.WriteByte(0xA0 | byte(n)) // tiny map
	} else if n <= 0xFF {
		buf.WriteByte(psMap8)
		buf.WriteByte(byte(n))
	} else {
		buf.WriteByte(psMap16)
		if err := binary.Write(buf, binary.BigEndian, uint16(n)); err != nil { //nolint:gosec // n > 0xFF, bounded by practical map size
			return err
		}
	}
	for k, v := range m {
		if err := packString(buf, k); err != nil {
			return err
		}
		if err := PackStreamEncode(buf, v); err != nil {
			return err
		}
	}
	return nil
}

// --- PackStream Decoder ---

// PackStreamDecode decodes a value from a PackStream reader.
func PackStreamDecode(r *bytes.Reader) (any, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	// Tiny int (positive): 0x00-0x7F
	if b <= 0x7F {
		return int64(b), nil
	}
	// Tiny int (negative): 0xF0-0xFF
	if b >= 0xF0 {
		return int64(int8(b)), nil //nolint:gosec // intentional sign extension for negative tiny int
	}
	// Tiny string: 0x80-0x8F
	if b >= 0x80 && b <= 0x8F {
		n := int(b & 0x0F)
		return readStringN(r, n)
	}
	// Tiny list: 0x90-0x9F
	if b >= 0x90 && b <= 0x9F {
		n := int(b & 0x0F)
		return readListN(r, n)
	}
	// Tiny map: 0xA0-0xAF
	if b >= 0xA0 && b <= 0xAF {
		n := int(b & 0x0F)
		return readMapN(r, n)
	}
	// Struct marker: 0xB0-0xBF
	if b >= 0xB0 && b <= 0xBF {
		nFields := int(b & 0x0F)
		sig, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		fields := make([]any, nFields)
		for i := range nFields {
			fields[i], err = PackStreamDecode(r)
			if err != nil {
				return nil, err
			}
		}
		return &BoltMessage{Type: sig, Fields: fields}, nil
	}

	switch b {
	case psNil:
		return nil, nil
	case psFalse:
		return false, nil
	case psTrue:
		return true, nil
	case psFloat64:
		var v float64
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return nil, err
		}
		return v, nil
	case psInt8:
		b2, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		return int64(int8(b2)), nil //nolint:gosec // intentional int8 decoding
	case psInt16:
		var v int16
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return nil, err
		}
		return int64(v), nil
	case psInt32:
		var v int32
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return nil, err
		}
		return int64(v), nil
	case psInt64:
		var v int64
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return nil, err
		}
		return v, nil
	case psString8:
		n, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		return readStringN(r, int(n))
	case psString16:
		var n uint16
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		return readStringN(r, int(n))
	case psString32:
		var n uint32
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		return readStringN(r, int(n))
	case psList8:
		n, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		return readListN(r, int(n))
	case psList16:
		var n uint16
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		return readListN(r, int(n))
	case psMap8:
		n, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		return readMapN(r, int(n))
	case psMap16:
		var n uint16
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		return readMapN(r, int(n))
	case psStruct8:
		n, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		sig, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		fields := make([]any, n)
		for i := range int(n) {
			fields[i], err = PackStreamDecode(r)
			if err != nil {
				return nil, err
			}
		}
		return &BoltMessage{Type: sig, Fields: fields}, nil
	}

	return nil, fmt.Errorf("packstream: unknown marker 0x%02X", b)
}

func readStringN(r *bytes.Reader, n int) (string, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readListN(r *bytes.Reader, n int) ([]any, error) {
	list := make([]any, n)
	for i := range n {
		v, err := PackStreamDecode(r)
		if err != nil {
			return nil, err
		}
		list[i] = v
	}
	return list, nil
}

func readMapN(r *bytes.Reader, n int) (map[string]any, error) {
	m := make(map[string]any, n)
	for range n {
		key, err := PackStreamDecode(r)
		if err != nil {
			return nil, err
		}
		val, err := PackStreamDecode(r)
		if err != nil {
			return nil, err
		}
		m[fmt.Sprint(key)] = val
	}
	return m, nil
}

// --- Chunked Transport ---

// WriteChunked writes a message in Bolt chunked format: [uint16 size][bytes]...[0x0000].
func WriteChunked(w io.Writer, data []byte) error {
	const maxChunk = 0xFFFF
	for len(data) > 0 {
		chunk := data
		if len(chunk) > maxChunk {
			chunk = data[:maxChunk]
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(chunk))); err != nil { //nolint:gosec // len(chunk) <= maxChunk (0xFFFF)
			return err
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
		data = data[len(chunk):]
	}
	// End marker
	return binary.Write(w, binary.BigEndian, uint16(0))
}

// ReadChunked reads a chunked message until the zero-length terminator.
func ReadChunked(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	for {
		var size uint16
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return nil, err
		}
		if size == 0 {
			break
		}
		chunk := make([]byte, size)
		if _, err := io.ReadFull(r, chunk); err != nil {
			return nil, err
		}
		buf.Write(chunk)
	}
	return buf.Bytes(), nil
}

// EncodeMessage encodes a BoltMessage into PackStream bytes.
func EncodeMessage(msgType byte, fields ...any) ([]byte, error) {
	var buf bytes.Buffer
	nFields := len(fields)
	if nFields > 15 {
		return nil, fmt.Errorf("bolt: too many fields for tiny struct: %d (max 15)", nFields)
	}
	buf.WriteByte(0xB0 | byte(nFields)) //nolint:gosec // nFields <= 15
	buf.WriteByte(msgType)
	for _, f := range fields {
		if err := PackStreamEncode(&buf, f); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// DecodeMessage reads and decodes a chunked Bolt message.
func DecodeMessage(r io.Reader) (*BoltMessage, error) {
	data, err := ReadChunked(r)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)
	v, err := PackStreamDecode(reader)
	if err != nil {
		return nil, err
	}
	msg, ok := v.(*BoltMessage)
	if !ok {
		return nil, fmt.Errorf("expected struct, got %T", v)
	}
	return msg, nil
}

// SendMessage encodes and writes a chunked Bolt message.
func SendMessage(w io.Writer, msgType byte, fields ...any) error {
	data, err := EncodeMessage(msgType, fields...)
	if err != nil {
		return err
	}
	return WriteChunked(w, data)
}
