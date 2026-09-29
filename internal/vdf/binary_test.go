package vdf

import (
	"encoding/binary"
	"testing"
)

type enc struct{ b []byte }

func (e *enc) str(k, v string) *enc {
	e.b = append(e.b, binString)
	e.b = append(append(e.b, k...), 0)
	e.b = append(append(e.b, v...), 0)
	return e
}

func (e *enc) u32(k string, v uint32) *enc {
	e.b = append(e.b, binInt32)
	e.b = append(append(e.b, k...), 0)
	e.b = binary.LittleEndian.AppendUint32(e.b, v)
	return e
}

func (e *enc) open(k string) *enc {
	e.b = append(e.b, binMap)
	e.b = append(append(e.b, k...), 0)
	return e
}

func (e *enc) end() *enc {
	e.b = append(e.b, binEnd)
	return e
}

func shortcuts() []byte {
	e := &enc{}
	e.open("shortcuts").
		open("0").u32("appid", 2501829966).str("AppName", "RetroDECK").
		open("tags").str("0", "RetroDECK").end().
		end().
		open("1").u32("appid", 42).str("AppName", "").end().
		end().
		end()
	return e.b
}

func TestParseBinary(t *testing.T) {
	n, err := ParseBinary(shortcuts())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := n.Get("shortcuts", "0", "appid"); got != "2501829966" {
		t.Errorf("appid = %q, want unsigned 2501829966", got)
	}
	if got, _ := n.Get("shortcuts", "0", "appname"); got != "RetroDECK" {
		t.Errorf("appname = %q", got)
	}
	if got, _ := n.Get("shortcuts", "0", "tags", "0"); got != "RetroDECK" {
		t.Errorf("tag = %q", got)
	}
	if got, ok := n.Get("shortcuts", "1", "appname"); !ok || got != "" {
		t.Errorf("empty appname = %q, %v", got, ok)
	}
}

func TestParseBinaryErrors(t *testing.T) {
	good := shortcuts()
	cases := map[string][]byte{
		"truncated string": good[:20],
		"truncated int":    (&enc{b: []byte{binMap, 'a', 0, binInt32, 'k', 0, 1, 2}}).b,
		"unknown type":     {binMap, 'a', 0, 0x05, 'k', 0, 'x', 0, binEnd, binEnd},
		"missing end":      (&enc{}).open("a").str("k", "v").b,
	}
	for name, in := range cases {
		if _, err := ParseBinary(in); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
