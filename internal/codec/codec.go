// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package codec converts between flag text and Go values of a runtime type.
//
// A Codec parses into fresh storage, so invalid input never commits a partial
// value. Codecs know nothing about flags, FlagSets, or occurrence state.
package codec

import (
	"encoding"
	"fmt"
	"reflect"
)

// Codec parses and formats one Go type.
type Codec struct {
	// TypeName is reported by Value.Type and shown in help. Named types report
	// their kind: "uint16" for a type Port uint16. Types with their own text
	// syntax report a fixed name: "duration", or "text" for a TextUnmarshaler.
	TypeName string

	Parse  func(string) (reflect.Value, error)
	Format func(reflect.Value) string

	Boolean     bool // a bare flag means true
	EmptyNoop   bool // Set("") leaves the value unchanged (net.IP)
	StripQuotes bool // in lists, quotes are never meaningful (bool, IP, IPNet)
	TrimSpace   bool // in lists, whitespace around fields is ignored
	ByKind      bool // chosen by the underlying kind, not by the type

	base reflect.Type // the Go type Parse returns and Format accepts
}

// For returns the codec for t. Precedence:
//  1. standard-library types with their usual syntax (Duration, Time, IP, ...)
//  2. t, or *t, implements encoding.TextUnmarshaler
//  3. t's underlying kind (bool, string, integers, floats, complex numbers)
func For(t reflect.Type) (Codec, bool) {
	c, ok := lookup(t)
	if ok {
		c.TrimSpace = t.Kind() != reflect.String
	}
	return c, ok
}

func lookup(t reflect.Type) (Codec, bool) {
	if c, ok := stdlibCodecs[t]; ok {
		return c, true
	}
	if c, ok := textPointerCodec(t); ok {
		return c, true
	}
	if c, ok := textCodec(t); ok {
		return c, true
	}
	if c, ok := kindCodecs[t.Kind()]; ok {
		c = c.as(t)
		c.ByKind = true
		return c, true
	}
	return Codec{}, false
}

// newCodec builds a codec for T from typed functions. A non-empty hint
// replaces the parse error's message while keeping it as the cause.
func newCodec[T any](name, hint string, parse func(string) (T, error), format func(T) string) Codec {
	return Codec{
		TypeName: name,
		base:     reflect.TypeFor[T](),
		Parse: func(s string) (reflect.Value, error) {
			v, err := parse(s)
			if err != nil {
				return reflect.Value{}, withHint(err, hint)
			}
			return reflect.ValueOf(&v).Elem(), nil
		},
		Format: func(v reflect.Value) string { return format(v.Interface().(T)) },
	}
}

// as adapts a codec for a builtin type to a named type of the same kind. The
// type name stays that of the kind.
func (c Codec) as(t reflect.Type) Codec {
	if t == c.base {
		return c
	}
	parse, format, base := c.Parse, c.Format, c.base
	c.base = t
	c.Parse = func(s string) (reflect.Value, error) {
		v, err := parse(s)
		if err != nil {
			return v, err
		}
		return v.Convert(t), nil
	}
	c.Format = func(v reflect.Value) string { return format(v.Convert(base)) }
	return c
}

// textCodec handles types whose pointer implements encoding.TextUnmarshaler.
func textCodec(t reflect.Type) (Codec, bool) {
	unmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()

	if !reflect.PointerTo(t).Implements(unmarshalerType) {
		return Codec{}, false
	}
	return Codec{
		TypeName: "text",
		base:     t,
		Parse: func(s string) (reflect.Value, error) {
			p := reflect.New(t)
			err := p.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s))
			return p.Elem(), err
		},
		Format: func(v reflect.Value) string {
			p := reflect.New(t)
			p.Elem().Set(v)
			return marshalText(p.Interface(), v.Interface())
		},
	}, true
}

// textPointerCodec handles pointer types, like *big.Int, that implement
// encoding.TextUnmarshaler themselves.
func textPointerCodec(t reflect.Type) (Codec, bool) {
	unmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()

	if t.Kind() != reflect.Pointer || !t.Implements(unmarshalerType) {
		return Codec{}, false
	}
	return Codec{
		TypeName: "text",
		base:     t,
		Parse: func(s string) (reflect.Value, error) {
			p := reflect.New(t.Elem())
			err := p.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s))
			return p, err
		},
		Format: func(v reflect.Value) string {
			if v.IsNil() {
				return ""
			}
			return marshalText(v.Interface(), v.Interface())
		},
	}, true
}

// marshalText formats m with MarshalText when it implements it, and fallback
// with fmt otherwise (types may implement only UnmarshalText).
func marshalText(m, fallback any) string {
	if m, ok := m.(encoding.TextMarshaler); ok {
		if b, err := m.MarshalText(); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(fallback)
}
