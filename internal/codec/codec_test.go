// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codec

import (
	"errors"
	"math/big"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"
)

type (
	namedBool    bool
	namedString  string
	namedInt8    int8
	namedUint16  uint16
	namedFloat32 float32
	namedComplex complex64
)

func TestKindRoundTrip(t *testing.T) {
	tests := []struct {
		value    any
		text     string
		typeName string
	}{
		{true, "true", "bool"},
		{namedBool(true), "true", "bool"},
		{"text", "text", "string"},
		{namedString("text"), "text", "string"},
		{-7, "-7", "int"},
		{int8(-8), "-8", "int8"},
		{namedInt8(-8), "-8", "int8"},
		{int16(-16), "-16", "int16"},
		{int32(-32), "-32", "int32"},
		{int64(-64), "-64", "int64"},
		{uint(7), "7", "uint"},
		{uint8(8), "8", "uint8"},
		{uint16(16), "16", "uint16"},
		{namedUint16(16), "16", "uint16"},
		{uint32(32), "32", "uint32"},
		{uint64(64), "64", "uint64"},
		{uintptr(9), "9", "uintptr"},
		{float32(1.5), "1.5", "float32"},
		{namedFloat32(1.5), "1.5", "float32"},
		{2.25, "2.25", "float64"},
		{complex64(1 + 2i), "(1+2i)", "complex64"},
		{namedComplex(1 + 2i), "(1+2i)", "complex64"},
		{complex128(1 + 2i), "(1+2i)", "complex128"},
	}
	for _, test := range tests {
		typ := reflect.TypeOf(test.value)
		c, ok := For(typ)
		if !ok {
			t.Fatalf("no codec for %s", typ)
		}
		if c.TypeName != test.typeName {
			t.Errorf("%s TypeName = %q, want %q", typ, c.TypeName, test.typeName)
		}
		v, err := c.Parse(test.text)
		if err != nil {
			t.Fatalf("%s Parse(%q): %v", typ, test.text, err)
		}
		if v.Type() != typ || v.Interface() != test.value {
			t.Errorf("%s Parse(%q) = %#v", typ, test.text, v.Interface())
		}
		if got := c.Format(v); got != test.text {
			t.Errorf("%s Format = %q, want %q", typ, got, test.text)
		}
		if c.TrimSpace != (typ.Kind() != reflect.String) {
			t.Errorf("%s TrimSpace = %v", typ, c.TrimSpace)
		}
	}
}

func TestKindRanges(t *testing.T) {
	c, _ := For(reflect.TypeFor[namedInt8]())
	_, err := c.Parse("128")
	if !errors.Is(err, strconv.ErrRange) || err.Error() != "out of range for int8" {
		t.Fatalf("out-of-range int8 = %v", err)
	}
}

func TestStdlibCodecs(t *testing.T) {
	tests := []struct {
		typ  reflect.Type
		text string
		name string
	}{
		{reflect.TypeFor[time.Duration](), "1m30s", "duration"},
		{reflect.TypeFor[time.Time](), "2026-11-18T10:00:00Z", "time"},
		{reflect.TypeFor[net.IP](), "10.0.0.1", "ip"},
		{reflect.TypeFor[net.IPMask](), "ffffff00", "ipMask"},
		{reflect.TypeFor[net.IPNet](), "10.0.0.0/8", "ipNet"},
	}
	for _, test := range tests {
		c, ok := For(test.typ)
		if !ok || c.TypeName != test.name {
			t.Fatalf("%s codec = %q, %v", test.typ, c.TypeName, ok)
		}
		v, err := c.Parse(test.text)
		if err != nil {
			t.Fatalf("%s Parse(%q): %v", test.typ, test.text, err)
		}
		if got := c.Format(v); got != test.text {
			t.Errorf("%s Format = %q, want %q", test.typ, got, test.text)
		}
	}
}

func TestListFlags(t *testing.T) {
	for typ, want := range map[reflect.Type][2]bool{
		reflect.TypeFor[bool]():      {false, true},
		reflect.TypeFor[net.IP]():    {true, true},
		reflect.TypeFor[net.IPNet](): {false, true},
		reflect.TypeFor[string]():    {false, false},
		reflect.TypeFor[int]():       {false, false},
	} {
		c, _ := For(typ)
		if got := [2]bool{c.EmptyNoop, c.StripQuotes}; got != want {
			t.Errorf("%s EmptyNoop, StripQuotes = %v, want %v", typ, got, want)
		}
	}
}

type unmarshalOnly struct{ text string }

func (u *unmarshalOnly) UnmarshalText(b []byte) error { u.text = string(b); return nil }

func TestTextCodecs(t *testing.T) {
	c, ok := For(reflect.TypeFor[*big.Int]())
	if !ok || c.TypeName != "text" {
		t.Fatalf("pointer text codec = %q, %v", c.TypeName, ok)
	}
	if v, _ := c.Parse("12345678901234567890"); c.Format(v) != "12345678901234567890" {
		t.Errorf("pointer text codec round trip = %q", c.Format(v))
	}
	if got := c.Format(reflect.Zero(reflect.TypeFor[*big.Int]())); got != "" {
		t.Errorf("nil pointer formats as %q", got)
	}

	c, ok = For(reflect.TypeFor[unmarshalOnly]())
	if !ok {
		t.Fatal("no codec for unmarshal-only type")
	}
	if v, _ := c.Parse("x"); c.Format(v) != "{x}" {
		t.Errorf("unmarshal-only fallback = %q", c.Format(v))
	}
}

func TestUnsupported(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[struct{}](), reflect.TypeFor[[]int](), reflect.TypeFor[chan int]()} {
		if _, ok := For(typ); ok {
			t.Errorf("unexpected codec for %s", typ)
		}
	}
}
