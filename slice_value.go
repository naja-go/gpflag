// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/naja-go/gpflag/internal/codec"
	"github.com/naja-go/gpflag/pkg/csv"
)

// sliceValue stores a []E where E has a scalar codec. This can accept CSV values.
//
// The first successful occurrence replaces the default; later occurrences
// append. For example, given
//
//	ports := fs.New("port", []uint16{8080}, "listen ports")
//
// the command line
//
//	--port=80,443 --port=8443
//
// yields []uint16{80, 443, 8443}: the default 8080 is dropped by the first
// occurrence, and the second occurrence appends. A CSV field may be quoted to
// hold a comma, so for a []string flag
//
//	--name='"Doe, Jane",John'
//
// yields []string{"Doe, Jane", "John"}. An occurrence that fails to parse
// changes nothing.
type sliceValue struct {
	dst      reflect.Value
	elem     codec.Codec
	replaced bool // the default has been replaced by a parsed occurrence
}

// listQuotes removes quotes from lists whose elements cannot contain them.
// pflag accepts quotes around whole boolean/network lists as well as elements.
var listQuotes = strings.NewReplacer(`"`, "", "'", "", "`", "")

func newSliceValue(dst reflect.Value) *sliceValue {
	elem, ok := codecFor(dst.Type().Elem())
	if !ok {
		panic(fmt.Sprintf("gpflag: unsupported slice element type %s", dst.Type().Elem()))
	}
	return &sliceValue{dst: dst, elem: elem}
}

func (v *sliceValue) Type() string        { return v.elem.TypeName + "Slice" }
func (v *sliceValue) zeroDefault() string { return "[]" }

func (v *sliceValue) Set(s string) error {
	if v.elem.StripQuotes {
		s = listQuotes.Replace(s)
	}
	fields, err := csv.Read(s)
	if err != nil {
		return err
	}
	parsed, err := v.parse(fields)
	if err != nil {
		return err
	}
	if v.replaced {
		parsed = reflect.AppendSlice(v.dst, parsed)
	}
	v.dst.Set(parsed)
	v.replaced = true
	return nil
}

// String tolerates the zero value, which the standard flag package creates.
func (v *sliceValue) String() string {
	if !v.dst.IsValid() {
		return ""
	}
	return "[" + csv.Format(v.GetSlice()) + "]"
}

func (v *sliceValue) Get() any { return v.dst.Interface() }

// Append implements SliceValue.
func (v *sliceValue) Append(s string) error {
	parsed, err := v.parse([]string{s})
	if err == nil {
		v.dst.Set(reflect.AppendSlice(v.dst, parsed))
	}
	return err
}

// Replace implements SliceValue.
func (v *sliceValue) Replace(fields []string) error {
	parsed, err := v.parse(fields)
	if err == nil {
		v.dst.Set(parsed)
	}
	return err
}

// GetSlice implements SliceValue.
func (v *sliceValue) GetSlice() []string {
	out := make([]string, v.dst.Len())
	for i := range out {
		out[i] = v.elem.Format(v.dst.Index(i))
	}
	return out
}

// parse converts fields into a fresh slice, committing nothing on error.
func (v *sliceValue) parse(fields []string) (reflect.Value, error) {
	out := reflect.MakeSlice(v.dst.Type(), len(fields), len(fields))
	for i, field := range fields {
		if v.elem.TrimSpace {
			field = strings.TrimSpace(field)
		}
		elem, err := v.elem.Parse(field)
		if err != nil {
			return reflect.Value{}, err
		}
		out.Index(i).Set(elem)
	}
	return out, nil
}
