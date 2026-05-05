// Package cbor decodes CBOR (RFC 8949 / STD 94) data into Go values.
//
// The decoder handles all major types defined in the spec:
//
//	0  Unsigned integer
//	1  Negative integer
//	2  Byte string
//	3  Text string
//	4  Array
//	5  Map
//	6  Tagged item  (tag is discarded; inner value is returned)
//	7  Floating-point, simple values, break
//
// Indefinite-length arrays, maps, byte strings, and text strings are
// supported. Bignum tags (2 and 3) are decoded to int64 on a best-effort
// basis. Unknown tags are unwrapped and their inner value returned.
//
// Resource limits: arrays/maps are limited to 1 000 000 items and recursion
// to 128 levels to prevent resource exhaustion from malicious input.
package cbor

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	maxItems     = 1_000_000
	maxDepth     = 128
	breakCode    = 0xff
)

// Unmarshal decodes CBOR-encoded data into a Go value.
// The result type depends on the CBOR major type:
//
//	uint (0)  → uint64
//	nint (1)  → int64
//	bstr (2)  → []byte   (also base64-encoded string in JSON output)
//	tstr (3)  → string
//	array(4)  → []any
//	map  (5)  → map[string]any
//	tagged(6) → inner value (tag discarded)
//	float(7)  → float64
//	bool/nil  → bool / nil
func Unmarshal(data []byte, v *any) error {
	if len(data) == 0 {
		return fmt.Errorf("empty CBOR data")
	}
	d := &decoder{buf: data}
	val, err := d.decode(0)
	if err != nil {
		return err
	}
	if d.pos < len(d.buf) {
		// trailing bytes are allowed by the spec (CBOR sequence) but warn via error
		// for simple unmarshal usage we just ignore them
	}
	*v = val
	return nil
}

type decoder struct {
	buf []byte
	pos int
}

func (d *decoder) decode(depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("CBOR: recursion limit exceeded")
	}
	if d.pos >= len(d.buf) {
		return nil, fmt.Errorf("CBOR: unexpected end of data")
	}

	b := d.buf[d.pos]
	d.pos++

	major := b >> 5
	info := b & 0x1f

	switch major {
	case 0: // unsigned integer
		n, err := d.readUint(info)
		if err != nil {
			return nil, err
		}
		return n, nil

	case 1: // negative integer: -1 - n
		n, err := d.readUint(info)
		if err != nil {
			return nil, err
		}
		if n > math.MaxInt64 {
			return nil, fmt.Errorf("CBOR: negative integer overflow")
		}
		return -1 - int64(n), nil

	case 2: // byte string
		return d.readBytes(info)

	case 3: // text string
		bs, err := d.readBytes(info)
		if err != nil {
			return nil, err
		}
		return string(bs), nil

	case 4: // array
		return d.readArray(info, depth)

	case 5: // map
		return d.readMap(info, depth)

	case 6: // tagged item
		tag, err := d.readUint(info)
		if err != nil {
			return nil, err
		}
		inner, err := d.decode(depth + 1)
		if err != nil {
			return nil, err
		}
		// Handle bignum tags 2 (positive) and 3 (negative)
		if tag == 2 {
			if bs, ok := inner.([]byte); ok {
				return decodeBignum(bs, false), nil
			}
		}
		if tag == 3 {
			if bs, ok := inner.([]byte); ok {
				return decodeBignum(bs, true), nil
			}
		}
		// For tag 0 (datetime string) and tag 1 (epoch datetime), just return inner.
		// For all other tags, unwrap.
		return inner, nil

	case 7: // float / simple / break
		return d.readSpecial(info)
	}

	return nil, fmt.Errorf("CBOR: unknown major type %d", major)
}

// readUint reads the argument value for the given additional info byte.
func (d *decoder) readUint(info byte) (uint64, error) {
	switch {
	case info <= 23:
		return uint64(info), nil
	case info == 24:
		if d.pos+1 > len(d.buf) {
			return 0, fmt.Errorf("CBOR: unexpected end reading uint8")
		}
		v := uint64(d.buf[d.pos])
		d.pos++
		return v, nil
	case info == 25:
		if d.pos+2 > len(d.buf) {
			return 0, fmt.Errorf("CBOR: unexpected end reading uint16")
		}
		v := uint64(binary.BigEndian.Uint16(d.buf[d.pos : d.pos+2]))
		d.pos += 2
		return v, nil
	case info == 26:
		if d.pos+4 > len(d.buf) {
			return 0, fmt.Errorf("CBOR: unexpected end reading uint32")
		}
		v := uint64(binary.BigEndian.Uint32(d.buf[d.pos : d.pos+4]))
		d.pos += 4
		return v, nil
	case info == 27:
		if d.pos+8 > len(d.buf) {
			return 0, fmt.Errorf("CBOR: unexpected end reading uint64")
		}
		v := binary.BigEndian.Uint64(d.buf[d.pos : d.pos+8])
		d.pos += 8
		return v, nil
	case info == 31:
		// Indefinite length — caller handles
		return 0, nil
	default:
		return 0, fmt.Errorf("CBOR: reserved additional info %d", info)
	}
}

// readBytes reads a definite or indefinite-length byte/text string.
func (d *decoder) readBytes(info byte) ([]byte, error) {
	if info == 31 { // indefinite length: sequence of definite-length chunks
		var out []byte
		for {
			if d.pos >= len(d.buf) {
				return nil, fmt.Errorf("CBOR: unexpected end in indefinite bytes")
			}
			if d.buf[d.pos] == breakCode {
				d.pos++
				break
			}
			// Each chunk is a definite-length byte/text string.
			hdr := d.buf[d.pos]
			d.pos++
			chunkInfo := hdr & 0x1f
			chunk, err := d.readBytes(chunkInfo)
			if err != nil {
				return nil, fmt.Errorf("CBOR: error reading indefinite chunk: %w", err)
			}
			out = append(out, chunk...)
		}
		return out, nil
	}
	n, err := d.readUint(info)
	if err != nil {
		return nil, err
	}
	if n > maxItems {
		return nil, fmt.Errorf("CBOR: byte string too long: %d", n)
	}
	sz := int(n)
	if d.pos+sz > len(d.buf) {
		return nil, fmt.Errorf("CBOR: unexpected end reading %d bytes", sz)
	}
	out := make([]byte, sz)
	copy(out, d.buf[d.pos:d.pos+sz])
	d.pos += sz
	return out, nil
}

// readArray reads a CBOR array (definite or indefinite length).
func (d *decoder) readArray(info byte, depth int) ([]any, error) {
	indefinite := info == 31
	var count int
	if !indefinite {
		n, err := d.readUint(info)
		if err != nil {
			return nil, err
		}
		if n > maxItems {
			return nil, fmt.Errorf("CBOR: array too large: %d", n)
		}
		count = int(n)
	}

	var out []any
	for i := 0; indefinite || i < count; i++ {
		if indefinite {
			if d.pos >= len(d.buf) {
				return nil, fmt.Errorf("CBOR: unexpected end in indefinite array")
			}
			if d.buf[d.pos] == breakCode {
				d.pos++
				break
			}
		}
		v, err := d.decode(depth + 1)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// readMap reads a CBOR map (definite or indefinite length).
// Keys are coerced to strings.
func (d *decoder) readMap(info byte, depth int) (map[string]any, error) {
	indefinite := info == 31
	var count int
	if !indefinite {
		n, err := d.readUint(info)
		if err != nil {
			return nil, err
		}
		if n > maxItems {
			return nil, fmt.Errorf("CBOR: map too large: %d", n)
		}
		count = int(n)
	}

	out := make(map[string]any)
	for i := 0; indefinite || i < count; i++ {
		if indefinite {
			if d.pos >= len(d.buf) {
				return nil, fmt.Errorf("CBOR: unexpected end in indefinite map")
			}
			if d.buf[d.pos] == breakCode {
				d.pos++
				break
			}
		}
		k, err := d.decode(depth + 1)
		if err != nil {
			return nil, err
		}
		v, err := d.decode(depth + 1)
		if err != nil {
			return nil, err
		}
		out[fmt.Sprintf("%v", k)] = v
	}
	return out, nil
}

// readSpecial decodes major type 7: floats, simple values, and break.
func (d *decoder) readSpecial(info byte) (any, error) {
	switch {
	case info == 20:
		return false, nil
	case info == 21:
		return true, nil
	case info == 22:
		return nil, nil
	case info == 23:
		return nil, nil // undefined → nil
	case info == 24: // simple value (1 byte)
		if d.pos+1 > len(d.buf) {
			return nil, fmt.Errorf("CBOR: unexpected end reading simple value")
		}
		sv := d.buf[d.pos]
		d.pos++
		return fmt.Sprintf("simple(%d)", sv), nil
	case info == 25: // float16
		if d.pos+2 > len(d.buf) {
			return nil, fmt.Errorf("CBOR: unexpected end reading float16")
		}
		bits := binary.BigEndian.Uint16(d.buf[d.pos : d.pos+2])
		d.pos += 2
		return float64(float16ToFloat32(bits)), nil
	case info == 26: // float32
		if d.pos+4 > len(d.buf) {
			return nil, fmt.Errorf("CBOR: unexpected end reading float32")
		}
		bits := binary.BigEndian.Uint32(d.buf[d.pos : d.pos+4])
		d.pos += 4
		return float64(math.Float32frombits(bits)), nil
	case info == 27: // float64
		if d.pos+8 > len(d.buf) {
			return nil, fmt.Errorf("CBOR: unexpected end reading float64")
		}
		bits := binary.BigEndian.Uint64(d.buf[d.pos : d.pos+8])
		d.pos += 8
		return math.Float64frombits(bits), nil
	case info == 31:
		return nil, fmt.Errorf("CBOR: unexpected break code")
	default:
		return fmt.Sprintf("simple(%d)", info), nil
	}
}

// float16ToFloat32 converts an IEEE 754 half-precision float to single-precision.
func float16ToFloat32(b uint16) float32 {
	sign := uint32(b>>15) << 31
	exp := uint32((b >> 10) & 0x1f)
	mant := uint32(b & 0x03ff)

	var f uint32
	switch exp {
	case 0:
		if mant == 0 {
			f = sign
		} else {
			// Denormalised
			exp = 1
			for mant&0x0400 == 0 {
				mant <<= 1
				exp--
			}
			mant &= 0x03ff
			f = sign | ((exp + 127 - 15) << 23) | (mant << 13)
		}
	case 31:
		f = sign | 0x7f800000 | (mant << 13)
	default:
		f = sign | ((exp + 127 - 15) << 23) | (mant << 13)
	}
	return math.Float32frombits(f)
}

// decodeBignum converts a bignum byte slice to int64 (best effort).
// Returns a base64 string for numbers that don't fit in int64.
func decodeBignum(bs []byte, negative bool) any {
	if len(bs) <= 8 {
		var v uint64
		for _, b := range bs {
			v = v<<8 | uint64(b)
		}
		if negative {
			return -1 - int64(v)
		}
		if v <= math.MaxInt64 {
			return int64(v)
		}
		return v
	}
	// Too large for int64 — return base64 representation
	prefix := "bignum:"
	if negative {
		prefix = "neg-bignum:"
	}
	return prefix + base64.StdEncoding.EncodeToString(bs)
}
