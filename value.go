// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"reflect"

	"github.com/naja-go/gpflag/internal/codec"
)

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
//
// Values may also implement optional interfaces that FlagSet consults:
// [Getter], [BoolFlag], [NoOptDefaulter] and [FirstSetter].
type Value interface {
	String() string
	Set(string) error
	Type() string
}

// Getter is a Value that exposes its typed value, like the standard library's
// flag.Getter. [Get] uses it for Values that are not themselves a *T.
type Getter interface {
	Value
	Get() any
}

// BoolFlag is a Value that may be given without an argument, meaning "true",
// and is shown without a value placeholder in help output.
type BoolFlag interface {
	Value
	IsBoolFlag() bool
}

// NoOptDefaulter is a Value that accepts the flag without an argument. When
// the flag is added to a FlagSet with no NoOptDefVal of its own, NoOptDefValue
// supplies it: the text parsed when the flag is given without an argument.
type NoOptDefaulter interface {
	Value
	NoOptDefValue() string
}

// FirstSetter is a Value that distinguishes the first successful occurrence of
// a flag from later ones, for example to replace a default list and then
// append to it. FlagSet.Set and Parse call SetFirst instead of Set, with first
// reporting whether the flag is not yet Changed. Calling Value.Set directly
// bypasses it.
type FirstSetter interface {
	Value
	SetFirst(s string, first bool) error
}

func isNoOptBoolValue(v Value) bool {
	b, ok := v.(BoolFlag)
	return ok && b.IsBoolFlag()
}

// SliceValue is a secondary interface to all flags which hold a list
// of values.  This allows full control over the value of list flags,
// and avoids complicated marshalling and unmarshalling to csv.
type SliceValue interface {
	// Append adds the specified value to the end of the flag value list.
	Append(string) error
	// Replace will fully overwrite any data currently in the flag value list.
	Replace([]string) error
	// GetSlice returns the flag value list as an array of strings.
	GetSlice() []string
}

// zeroDefaulter is implemented by Values that know the canonical text of their
// zero value. Help output uses it to omit zero defaults; other Values fall
// back to heuristics.
type zeroDefaulter interface {
	zeroDefault() string
}

// pointerer is implemented by Values that parse into a single Go variable.
// CopyToGoFlagSet uses the pointer to register native go flags.
type pointerer interface {
	pointer() any
}

// eraseValue picks the Value that parses into dst. Precedence:
//  1. *T implements Value: the destination is its own Value, used as is
//  2. T is a "func(string) error" callback
//  3. T has a scalar codec: see codecFor
//  4. []E or map[~string]E where E has a scalar codec
//
// Unsupported types panic.
func eraseValue(dst reflect.Value) Value {
	if v, ok := dst.Addr().Interface().(Value); ok {
		return v
	}
	if v, ok := newFuncValue(dst); ok {
		return v
	}
	if v, ok := newScalarValue(dst); ok {
		return v
	}
	switch dst.Kind() {
	case reflect.Slice:
		return newSliceValue(dst)
	case reflect.Map:
		return newMapValue(dst)
	}
	panic(fmt.Sprintf("gpflag: unsupported flag type %s", dst.Type()))
}

// codecFor returns the codec that parses and formats a single t: a scalar
// flag, a slice element or a map element. A type whose pointer implements
// Value parses itself; every other type is package codec's business.
func codecFor(t reflect.Type) (codec.Codec, bool) {
	if c, ok := valueCodec(t); ok {
		return c, true
	}
	return codec.For(t)
}

var valueType = reflect.TypeFor[Value]()

// valueCodec adapts a type whose pointer implements Value, so that it can be
// an element of a slice or map flag.
func valueCodec(t reflect.Type) (codec.Codec, bool) {
	if !reflect.PointerTo(t).Implements(valueType) {
		return codec.Codec{}, false
	}
	return codec.Codec{
		TypeName:  reflect.New(t).Interface().(Value).Type(),
		TrimSpace: t.Kind() != reflect.String,
		Parse: func(s string) (reflect.Value, error) {
			p := reflect.New(t)
			err := p.Interface().(Value).Set(s)
			return p.Elem(), err
		},
		Format: func(v reflect.Value) string {
			p := reflect.New(t)
			p.Elem().Set(v)
			return p.Interface().(Value).String()
		},
	}, true
}

// funcValue wraps a callback invoked on every occurrence.
type funcValue struct {
	dst reflect.Value
}

var callbackType = reflect.TypeFor[func(string) error]()

func newFuncValue(dst reflect.Value) (Value, bool) {
	if dst.Kind() != reflect.Func || !dst.Type().ConvertibleTo(callbackType) {
		return nil, false
	}
	return &funcValue{dst: dst}, true
}

func (v *funcValue) Set(s string) error {
	if v.dst.IsNil() {
		return fmt.Errorf("gpflag: nil callback")
	}
	return v.dst.Convert(callbackType).Interface().(func(string) error)(s)
}

func (v *funcValue) String() string { return "" }
func (v *funcValue) Type() string   { return "func" }
func (v *funcValue) Get() any       { return v.dst.Interface() }

// scalarValue stores a single codec-parsed value.
type scalarValue struct {
	dst   reflect.Value
	codec codec.Codec
}

func newScalarValue(dst reflect.Value) (Value, bool) {
	c, ok := codecFor(dst.Type())
	if !ok {
		return nil, false
	}
	return &scalarValue{dst: dst, codec: c}, true
}

func (v *scalarValue) Set(s string) error {
	if v.codec.EmptyNoop && s == "" {
		return nil
	}
	parsed, err := v.codec.Parse(s)
	if err != nil {
		return err
	}
	v.dst.Set(parsed)
	return nil
}

// String tolerates the zero value, which the standard flag package creates.
func (v *scalarValue) String() string {
	if !v.dst.IsValid() {
		return ""
	}
	return v.codec.Format(v.dst)
}

func (v *scalarValue) Type() string     { return v.codec.TypeName }
func (v *scalarValue) IsBoolFlag() bool { return v.codec.Boolean }
func (v *scalarValue) Get() any         { return v.dst.Interface() }
func (v *scalarValue) pointer() any     { return v.dst.Addr().Interface() }

func (v *scalarValue) zeroDefault() string {
	return v.codec.Format(reflect.Zero(v.dst.Type()))
}
