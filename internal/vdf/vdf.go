// Package vdf parses Valve's text KeyValues format, as used by
// appmanifest_*.acf and libraryfolders.vdf.
//
// Only the subset needed here is supported: quoted or bare tokens,
// nested blocks in braces, and // line comments. Conditionals like
// [$WIN32] are not supported.
package vdf

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Values are string or Node. Keys are lowercased because Valve treats them
// case-insensitively. If a key repeats, the last value wins.
type Node map[string]any

func (n Node) Get(keys ...string) (string, bool) {
	cur := n
	for i, k := range keys {
		v, ok := cur[strings.ToLower(k)]
		if !ok {
			return "", false
		}
		if i == len(keys)-1 {
			s, ok := v.(string)
			return s, ok
		}
		if cur, ok = v.(Node); !ok {
			return "", false
		}
	}
	return "", false
}

func (n Node) Child(key string) (Node, bool) {
	c, ok := n[strings.ToLower(key)].(Node)
	return c, ok
}

func ParseFile(path string) (Node, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	n, err := Parse(string(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return n, nil
}

func Parse(s string) (Node, error) {
	p := &parser{src: s}
	root, err := p.block(false)
	if err != nil {
		return nil, fmt.Errorf("line %d: %w", p.line+1, err)
	}
	return root, nil
}

type parser struct {
	src  string
	pos  int
	line int
}

type tokKind int

const (
	tokEOF tokKind = iota
	tokString
	tokOpen
	tokClose
)

func (p *parser) block(nested bool) (Node, error) {
	n := Node{}
	for {
		kind, key, err := p.next()
		if err != nil {
			return nil, err
		}
		switch kind {
		case tokEOF:
			if nested {
				return nil, errors.New("unexpected end of input, missing '}'")
			}
			return n, nil
		case tokClose:
			if !nested {
				return nil, errors.New("unexpected '}'")
			}
			return n, nil
		case tokOpen:
			return nil, errors.New("unexpected '{', expected a key")
		}

		kind, val, err := p.next()
		if err != nil {
			return nil, err
		}
		switch kind {
		case tokString:
			n[strings.ToLower(key)] = val
		case tokOpen:
			child, err := p.block(true)
			if err != nil {
				return nil, err
			}
			n[strings.ToLower(key)] = child
		default:
			return nil, fmt.Errorf("key %q has no value", key)
		}
	}
}

func (p *parser) next() (tokKind, string, error) {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '\n':
			p.line++
			p.pos++
		case c == ' ' || c == '\t' || c == '\r':
			p.pos++
		case strings.HasPrefix(p.src[p.pos:], "//"):
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		case c == '{':
			p.pos++
			return tokOpen, "", nil
		case c == '}':
			p.pos++
			return tokClose, "", nil
		case c == '"':
			return p.quoted()
		default:
			return p.bare()
		}
	}
	return tokEOF, "", nil
}

func (p *parser) quoted() (tokKind, string, error) {
	p.pos++
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch c {
		case '"':
			p.pos++
			return tokString, b.String(), nil
		case '\\':
			if p.pos+1 < len(p.src) {
				p.pos++
				switch e := p.src[p.pos]; e {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(e)
				}
			}
		case '\n':
			p.line++
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		p.pos++
	}
	return 0, "", errors.New("unterminated string")
}

func (p *parser) bare() (tokKind, string, error) {
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '{' || c == '}' || c == '"' {
			break
		}
		p.pos++
	}
	return tokString, p.src[start:p.pos], nil
}
