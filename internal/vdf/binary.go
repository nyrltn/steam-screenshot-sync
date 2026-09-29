package vdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Binary KeyValues type bytes, as used by shortcuts.vdf.
const (
	binMap     = 0x00
	binString  = 0x01
	binInt32   = 0x02
	binFloat32 = 0x03
	binUint64  = 0x07
	binEnd     = 0x08
	binInt64   = 0x0A
	binEndAlt  = 0x0B
)

func ParseBinaryFile(path string) (Node, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	n, err := ParseBinary(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return n, nil
}

// Integers become decimal strings so Get works as for text files.
// int32 values are formatted unsigned, because Steam appids use the high bit.
func ParseBinary(b []byte) (Node, error) {
	p := &binParser{b: b}
	n, err := p.block(false)
	if err != nil {
		return nil, fmt.Errorf("offset %d: %w", p.pos, err)
	}
	return n, nil
}

type binParser struct {
	b   []byte
	pos int
}

func (p *binParser) block(nested bool) (Node, error) {
	n := Node{}
	for {
		if p.pos >= len(p.b) {
			if nested {
				return nil, fmt.Errorf("unexpected end of input, missing end marker")
			}
			return n, nil
		}
		t := p.b[p.pos]
		p.pos++
		if t == binEnd || t == binEndAlt {
			// Steam writes a trailing end marker after the root map too.
			return n, nil
		}
		key, err := p.cstring()
		if err != nil {
			return nil, err
		}
		key = strings.ToLower(key)
		switch t {
		case binMap:
			child, err := p.block(true)
			if err != nil {
				return nil, err
			}
			n[key] = child
		case binString:
			s, err := p.cstring()
			if err != nil {
				return nil, err
			}
			n[key] = s
		case binInt32:
			v, err := p.fixed(4)
			if err != nil {
				return nil, err
			}
			n[key] = strconv.FormatUint(uint64(binary.LittleEndian.Uint32(v)), 10)
		case binFloat32:
			v, err := p.fixed(4)
			if err != nil {
				return nil, err
			}
			f := math.Float32frombits(binary.LittleEndian.Uint32(v))
			n[key] = strconv.FormatFloat(float64(f), 'g', -1, 32)
		case binUint64:
			v, err := p.fixed(8)
			if err != nil {
				return nil, err
			}
			n[key] = strconv.FormatUint(binary.LittleEndian.Uint64(v), 10)
		case binInt64:
			v, err := p.fixed(8)
			if err != nil {
				return nil, err
			}
			n[key] = strconv.FormatInt(int64(binary.LittleEndian.Uint64(v)), 10)
		default:
			return nil, fmt.Errorf("unsupported type byte 0x%02x for key %q", t, key)
		}
	}
}

func (p *binParser) cstring() (string, error) {
	i := bytes.IndexByte(p.b[p.pos:], 0)
	if i < 0 {
		return "", fmt.Errorf("unterminated string")
	}
	s := string(p.b[p.pos : p.pos+i])
	p.pos += i + 1
	return s, nil
}

func (p *binParser) fixed(size int) ([]byte, error) {
	if p.pos+size > len(p.b) {
		return nil, fmt.Errorf("truncated value")
	}
	v := p.b[p.pos : p.pos+size]
	p.pos += size
	return v, nil
}
