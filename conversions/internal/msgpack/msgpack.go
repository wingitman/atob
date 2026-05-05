// Package msgpack decodes MessagePack-encoded data into Go values.
//
// It implements the full MessagePack specification
// (https://msgpack.org/index.html), including:
//
//   - All integer formats (positive/negative fixint, int8/16/32/64, uint8/16/32/64)
//   - Floating-point (float32, float64)
//   - Boolean, nil
//   - fixstr, str8/16/32
//   - bin8/16/32 (binary data → []byte)
//   - fixarray, array16/32
//   - fixmap, map16/32
//   - ext8/16/32, fixext1/2/4/8/16 (extension types → map with type/data keys)
//
// Resource limits: arrays/maps capped at 1 000 000 items; recursion at 128 levels.
package msgpack

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	maxItems = 1_000_000
	maxDepth = 128
)

// Unmarshal decodes MessagePack data and stores the result in v.
func Unmarshal(data []byte, v *any) error {
	if len(data) == 0 {
		return fmt.Errorf("empty MessagePack data")
	}
	d := &decoder{buf: data, pos: 0}
	val, err := d.decode(0)
	if err != nil {
		return err
	}
	*v = val
	return nil
}

type decoder struct {
	buf []byte
	pos int
}

func (d *decoder) readByte() (byte, error) {
	if d.pos >= len(d.buf) {
		return 0, fmt.Errorf("msgpack: unexpected end of data")
	}
	b := d.buf[d.pos]
	d.pos++
	return b, nil
}

func (d *decoder) readN(n int) ([]byte, error) {
	if d.pos+n > len(d.buf) {
		return nil, fmt.Errorf("msgpack: need %d bytes, have %d", n, len(d.buf)-d.pos)
	}
	out := d.buf[d.pos : d.pos+n]
	d.pos += n
	return out, nil
}

func (d *decoder) decode(depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("msgpack: recursion depth exceeded")
	}

	b, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch {
	// Positive fixint: 0xxxxxxx (0x00–0x7f)
	case b <= 0x7f:
		return uint64(b), nil

	// Fixmap: 1000xxxx (0x80–0x8f)
	case b >= 0x80 && b <= 0x8f:
		return d.readMap(int(b&0x0f), depth)

	// Fixarray: 1001xxxx (0x90–0x9f)
	case b >= 0x90 && b <= 0x9f:
		return d.readArray(int(b&0x0f), depth)

	// Fixstr: 101xxxxx (0xa0–0xbf)
	case b >= 0xa0 && b <= 0xbf:
		return d.readString(int(b & 0x1f))

	// nil
	case b == 0xc0:
		return nil, nil

	// (never used 0xc1)
	case b == 0xc1:
		return nil, fmt.Errorf("msgpack: unused format 0xc1")

	// false / true
	case b == 0xc2:
		return false, nil
	case b == 0xc3:
		return true, nil

	// bin8 / bin16 / bin32
	case b == 0xc4:
		raw, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return d.readBytes(int(raw))
	case b == 0xc5:
		bs, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.readBytes(int(binary.BigEndian.Uint16(bs)))
	case b == 0xc6:
		bs, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.readBytes(int(binary.BigEndian.Uint32(bs)))

	// ext8 / ext16 / ext32
	case b == 0xc7:
		raw, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return d.readExt(int(raw))
	case b == 0xc8:
		bs, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.readExt(int(binary.BigEndian.Uint16(bs)))
	case b == 0xc9:
		bs, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.readExt(int(binary.BigEndian.Uint32(bs)))

	// float32 / float64
	case b == 0xca:
		bs, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return float64(math.Float32frombits(binary.BigEndian.Uint32(bs))), nil
	case b == 0xcb:
		bs, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(bs)), nil

	// uint8 / uint16 / uint32 / uint64
	case b == 0xcc:
		raw, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return uint64(raw), nil
	case b == 0xcd:
		bs, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return uint64(binary.BigEndian.Uint16(bs)), nil
	case b == 0xce:
		bs, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return uint64(binary.BigEndian.Uint32(bs)), nil
	case b == 0xcf:
		bs, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return binary.BigEndian.Uint64(bs), nil

	// int8 / int16 / int32 / int64
	case b == 0xd0:
		raw, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return int64(int8(raw)), nil
	case b == 0xd1:
		bs, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return int64(int16(binary.BigEndian.Uint16(bs))), nil
	case b == 0xd2:
		bs, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return int64(int32(binary.BigEndian.Uint32(bs))), nil
	case b == 0xd3:
		bs, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint64(bs)), nil

	// fixext 1 / 2 / 4 / 8 / 16
	case b == 0xd4:
		return d.readExt(1)
	case b == 0xd5:
		return d.readExt(2)
	case b == 0xd6:
		return d.readExt(4)
	case b == 0xd7:
		return d.readExt(8)
	case b == 0xd8:
		return d.readExt(16)

	// str8 / str16 / str32
	case b == 0xd9:
		raw, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return d.readString(int(raw))
	case b == 0xda:
		bs, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.readString(int(binary.BigEndian.Uint16(bs)))
	case b == 0xdb:
		bs, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.readString(int(binary.BigEndian.Uint32(bs)))

	// array16 / array32
	case b == 0xdc:
		bs, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.readArray(int(binary.BigEndian.Uint16(bs)), depth)
	case b == 0xdd:
		bs, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.readArray(int(binary.BigEndian.Uint32(bs)), depth)

	// map16 / map32
	case b == 0xde:
		bs, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.readMap(int(binary.BigEndian.Uint16(bs)), depth)
	case b == 0xdf:
		bs, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.readMap(int(binary.BigEndian.Uint32(bs)), depth)

	// Negative fixint: 111xxxxx (0xe0–0xff)
	case b >= 0xe0:
		return int64(int8(b)), nil
	}

	return nil, fmt.Errorf("msgpack: unknown format byte 0x%02x", b)
}

func (d *decoder) readString(n int) (string, error) {
	if n > maxItems {
		return "", fmt.Errorf("msgpack: string too long: %d", n)
	}
	bs, err := d.readN(n)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func (d *decoder) readBytes(n int) ([]byte, error) {
	if n > maxItems {
		return nil, fmt.Errorf("msgpack: binary too long: %d", n)
	}
	bs, err := d.readN(n)
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, bs)
	return out, nil
}

func (d *decoder) readArray(count int, depth int) ([]any, error) {
	if count > maxItems {
		return nil, fmt.Errorf("msgpack: array too large: %d", count)
	}
	out := make([]any, count)
	for i := 0; i < count; i++ {
		v, err := d.decode(depth + 1)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (d *decoder) readMap(count int, depth int) (map[string]any, error) {
	if count > maxItems {
		return nil, fmt.Errorf("msgpack: map too large: %d", count)
	}
	out := make(map[string]any, count)
	for i := 0; i < count; i++ {
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

// readExt reads an extension type. The type byte comes first, then n data bytes.
func (d *decoder) readExt(n int) (map[string]any, error) {
	typeByte, err := d.readByte()
	if err != nil {
		return nil, err
	}
	data, err := d.readBytes(n)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ext_type": int(int8(typeByte)),
		"data":     data,
	}, nil
}
