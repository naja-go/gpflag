// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codec

import (
	"errors"
	"reflect"
	"strconv"
)

// kindCodecs holds one codec per builtin kind. Named types reuse them through
// Codec.as, so only the conversion uses reflection.
var kindCodecs = map[reflect.Kind]Codec{
	reflect.Bool:   boolCodec(),
	reflect.String: newCodec("string", "", func(s string) (string, error) { return s, nil }, func(v string) string { return v }),

	reflect.Int:   signed[int](),
	reflect.Int8:  signed[int8](),
	reflect.Int16: signed[int16](),
	reflect.Int32: signed[int32](),
	reflect.Int64: signed[int64](),

	reflect.Uint:    unsigned[uint](),
	reflect.Uint8:   unsigned[uint8](),
	reflect.Uint16:  unsigned[uint16](),
	reflect.Uint32:  unsigned[uint32](),
	reflect.Uint64:  unsigned[uint64](),
	reflect.Uintptr: unsigned[uintptr](),

	reflect.Float32: float[float32](),
	reflect.Float64: float[float64](),

	reflect.Complex64:  complexNumber[complex64](),
	reflect.Complex128: complexNumber[complex128](),
}

func boolCodec() Codec {
	c := newCodec("bool", "must be true or false", strconv.ParseBool, strconv.FormatBool)
	c.Boolean, c.StripQuotes = true, true
	return c
}

func signed[T ~int | ~int8 | ~int16 | ~int32 | ~int64]() Codec {
	t := reflect.TypeFor[T]()
	return newCodec(t.String(), "must be an integer",
		func(s string) (T, error) {
			v, err := strconv.ParseInt(s, 0, t.Bits())
			return T(v), rangeHint(err, t)
		},
		func(v T) string { return strconv.FormatInt(int64(v), 10) })
}

func unsigned[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr]() Codec {
	t := reflect.TypeFor[T]()
	return newCodec(t.String(), "must be a non-negative integer",
		func(s string) (T, error) {
			v, err := strconv.ParseUint(s, 0, t.Bits())
			return T(v), rangeHint(err, t)
		},
		func(v T) string { return strconv.FormatUint(uint64(v), 10) })
}

func float[T ~float32 | ~float64]() Codec {
	t := reflect.TypeFor[T]()
	return newCodec(t.String(), "must be a number",
		func(s string) (T, error) {
			v, err := strconv.ParseFloat(s, t.Bits())
			return T(v), rangeHint(err, t)
		},
		func(v T) string { return strconv.FormatFloat(float64(v), 'g', -1, t.Bits()) })
}

func complexNumber[T ~complex64 | ~complex128]() Codec {
	t := reflect.TypeFor[T]()
	return newCodec(t.String(), "",
		func(s string) (T, error) {
			v, err := strconv.ParseComplex(s, t.Bits())
			return T(v), err
		},
		func(v T) string { return strconv.FormatComplex(complex128(v), 'g', -1, t.Bits()) })
}

// rangeHint explains a well-formed number that does not fit t. The generic
// hint ("must be an integer") would be wrong for it.
func rangeHint(err error, t reflect.Type) error {
	if errors.Is(err, strconv.ErrRange) {
		return withHint(err, "out of range for "+t.String())
	}
	return err
}
