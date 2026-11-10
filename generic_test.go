// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag_test

import (
	"bytes"
	"errors"
	goflag "flag"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	flag "github.com/naja-go/gpflag"
	"github.com/naja-go/gpflag/pkg/types"
)

func checkGenericScalar[T comparable](t *testing.T, initial T, text string, want T, invalid ...string) {
	t.Helper()
	t.Run(reflect.TypeFor[T]().String(), func(t *testing.T) {
		fs := flag.NewFlagSet("scalar", flag.ContinueOnError)
		var value T
		f := fs.NewVarP(&value, "value", "v", initial, "a value")
		if value != initial || f.DefValue != f.Value.String() {
			t.Fatalf("default not initialized: value=%v, flag=%+v", value, f)
		}
		if err := fs.Parse([]string{"-v", text}); err != nil {
			t.Fatal(err)
		}
		got, err := fs.TryGet[T]("value")
		if err != nil || got != want || value != want || !f.Changed {
			t.Fatalf("got %v (bound %v), %v; want %v", got, value, err, want)
		}
		for _, text := range invalid {
			if err := fs.Set("value", text); err == nil {
				t.Errorf("accepted invalid value %q", text)
			}
			if value != want {
				t.Fatalf("failed parsing changed value to %v", value)
			}
		}
		// Typed retrieval observes storage directly, including external changes.
		value = initial
		got, err = f.TryGet[T]()
		if err != nil || got != initial {
			t.Fatalf("typed retrieval lost storage binding: %v, %v", got, err)
		}
	})
}

func TestGenericNativeScalars(t *testing.T) {
	checkGenericScalar(t, false, "true", true, "invalid")
	checkGenericScalar(t, "default", "hello, world", "hello, world")
	checkGenericScalar(t, int(7), "0x2a", int(42), "bad", "18446744073709551616")
	checkGenericScalar(t, int8(7), "-128", int8(-128), "128", "-129")
	checkGenericScalar(t, int16(7), "32767", int16(32767), "32768", "-32769")
	checkGenericScalar(t, int32(7), "2147483647", int32(2147483647), "2147483648", "-2147483649")
	checkGenericScalar(t, int64(7), "9223372036854775807", int64(9223372036854775807), "9223372036854775808", "-9223372036854775809")
	checkGenericScalar(t, uint(7), "0o52", uint(42), "-1", "18446744073709551616")
	checkGenericScalar(t, uint8(7), "255", uint8(255), "256", "-1")
	checkGenericScalar(t, uint16(7), "65535", uint16(65535), "65536", "-1")
	checkGenericScalar(t, uint32(7), "4294967295", uint32(4294967295), "4294967296", "-1")
	checkGenericScalar(t, uint64(7), "18446744073709551615", uint64(18446744073709551615), "18446744073709551616", "-1")
	checkGenericScalar(t, uintptr(7), "42", uintptr(42), "-1", "18446744073709551616")
	checkGenericScalar(t, float32(7), "1.25", float32(1.25), "bad", "1e100")
	checkGenericScalar(t, float64(7), "1.25", float64(1.25), "bad", "1e1000")
	checkGenericScalar(t, complex64(7), "(1+2i)", complex64(1+2i), "bad", "1e100i")
	checkGenericScalar(t, complex128(7), "(1+2i)", complex128(1+2i), "bad", "1e1000i")
}

func TestGenericNativeWordBounds(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	checkGenericScalar(t, 0, strconv.Itoa(maxInt), maxInt, strconv.FormatUint(uint64(maxInt)+1, 10))
	maxUint := ^uint(0)
	overflow := "18446744073709551616"
	if strconv.IntSize == 32 {
		overflow = "4294967296"
	}
	checkGenericScalar(t, uint(0), strconv.FormatUint(uint64(maxUint), 10), maxUint, overflow)
	checkGenericScalar(t, uintptr(0), strconv.FormatUint(uint64(maxUint), 10), uintptr(maxUint), overflow)
}

// newSet returns an empty flag set for tests that need a single flag.
func newSet() *flag.FlagSet {
	return flag.NewFlagSet("test", flag.ContinueOnError)
}

// newFlag registers a flag in fs and returns the Flag, where New would return
// a pointer to its value.
func newFlag[T any](fs *flag.FlagSet, name string, value T, usage string) *flag.Flag {
	return fs.NewVar(new(T), name, value, usage)
}

// newFlagP is like newFlag, with a one-letter shorthand.
func newFlagP[T any](fs *flag.FlagSet, name, shorthand string, value T, usage string) *flag.Flag {
	return fs.NewVarP(new(T), name, shorthand, value, usage)
}

func TestGenericConstructors(t *testing.T) {
	var bound, shortBound int
	fs := flag.NewFlagSet("constructors", flag.ContinueOnError)
	other := flag.NewFlagSet("other", flag.ContinueOnError)
	flags := []*flag.Flag{
		newFlag(fs, "plain", 7, "a value"),
		newFlagP(fs, "short", "s", 8, ""),
		fs.NewVar(&bound, "bound", 9, ""),
		fs.NewVarP(&shortBound, "bound-short", "b", 10, ""),
	}
	for _, f := range flags {
		if fs.Lookup(f.Name) != f {
			t.Fatalf("constructor did not register %q", f.Name)
		}
		if other.Lookup(f.Name) != nil {
			t.Fatalf("constructor leaked %q into another flag set", f.Name)
		}
	}
	if bound != 9 || shortBound != 10 {
		t.Fatal("constructors did not initialize bound variables")
	}
	if fs.ShorthandLookup("s") != flags[1] || fs.ShorthandLookup("b") != flags[3] {
		t.Fatal("constructors did not register shorthands")
	}
	if err := fs.Parse([]string{"--plain=41", "--bound=42", "-b44", "-s45"}); err != nil {
		t.Fatal(err)
	}
	if bound != 42 || shortBound != 44 {
		t.Fatalf("lost binding: %d %d", bound, shortBound)
	}
	if got, err := flags[0].TryGet[int](); err != nil || got != 41 {
		t.Fatalf("New value = %d, %v; want 41", got, err)
	}
	if got, err := flags[1].TryGet[int](); err != nil || got != 45 {
		t.Fatalf("NewP value = %d, %v; want 45", got, err)
	}
}

func TestGenericBoolAndUsage(t *testing.T) {
	fs := flag.NewFlagSet("bools", flag.ContinueOnError)
	a := newFlagP(fs, "alpha", "a", false, "enable alpha")
	b := newFlagP(fs, "beta", "b", false, "enable beta")
	fs.New("empty", "", "empty string")
	fs.New("zero", 0, "zero integer")
	fs.New("word", "hello", "a word")
	fs.New("string-zero", "0", "not a zero value")
	fs.New("string-false", "false", "not a zero value")
	fs.New("complex-zero", complex128(0), "zero complex")
	if err := fs.Parse([]string{"-ab", "--alpha=false"}); err != nil {
		t.Fatal(err)
	}
	av, _ := a.TryGet[bool]()
	bv, _ := b.TryGet[bool]()
	if av || !bv || a.NoOptDefVal != "true" {
		t.Fatal("boolean no-option or combined shorthand behavior broken")
	}
	usage := fs.FlagUsages()
	if strings.Contains(usage, "(default false)") || strings.Contains(usage, "(default 0)") || strings.Contains(usage, `(default "")`) || strings.Contains(usage, "(default (0+0i))") {
		t.Fatalf("zero defaults should be omitted:\n%s", usage)
	}
	for _, text := range []string{`(default "hello")`, `(default "0")`, `(default "false")`} {
		if !strings.Contains(usage, text) {
			t.Fatalf("string default %s missing:\n%s", text, usage)
		}
	}
}

func TestGenericGetErrors(t *testing.T) {
	f := newFlag(newSet(), "number", int32(42), "")
	if _, err := flag.TryGet[int](f); err == nil {
		t.Fatal("Get converted instead of asserting the exact type")
	}
	if _, err := flag.TryGet[int](nil); err == nil {
		t.Fatal("Get accepted a nil flag")
	}
	if _, err := flag.TryGet[int](&flag.Flag{Name: "empty"}); err == nil {
		t.Fatal("Get accepted a value without a getter")
	}
	fs := flag.NewFlagSet("missing", flag.ContinueOnError)
	if _, err := fs.TryGet[int]("missing"); err == nil {
		t.Fatal("Get accepted an undefined flag")
	}
}

func TestGenericGetPreservesFloatBits(t *testing.T) {
	bits := uint64(0x7ff8000000000042) // A NaN with a payload lost by string formatting.
	got, err := flag.TryGet[float64](newFlag(newSet(), "nan", math.Float64frombits(bits), ""))
	if err != nil || math.Float64bits(got) != bits {
		t.Fatalf("Get round-tripped through text: %x, %v", math.Float64bits(got), err)
	}
}

func TestGenericConstructionPanics(t *testing.T) {
	for _, test := range []struct {
		name string
		call func()
	}{
		{"nil", func() { newSet().NewVar(nil, "nil", 0, "") }},
		{"unsupported", func() { newSet().New("unsupported", make(chan int), "") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected a construction-time panic")
				}
			}()
			test.call()
		})
	}
}

func TestGenericSpecialTypes(t *testing.T) {
	fs := flag.NewFlagSet("special", flag.ContinueOnError)
	var count types.Count
	f := fs.NewVarP(&count, "verbose", "v", types.Count(0), "verbosity")
	b := newFlag(fs, "data", types.Base64("default"), "binary data")
	if f.NoOptDefVal != "+1" {
		t.Fatal("custom no-option default lost")
	}
	if err := fs.Parse([]string{"-vvv", "--data=aGVsbG8="}); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
	got, err := b.TryGet[types.Base64]()
	if err != nil || string(got) != "hello" {
		t.Fatalf("base64 = %q, %v", got, err)
	}
	if err := fs.Set("data", "invalid!"); err == nil {
		t.Fatal("accepted invalid base64")
	}
	got, _ = b.TryGet[types.Base64]()
	if string(got) != "hello" {
		t.Fatal("failed base64 parse changed storage")
	}
	if err := fs.Set("verbose", "10"); err != nil || count != 10 {
		t.Fatalf("explicit count = %d, %v", count, err)
	}
}

// Exercise extension hooks from outside gpflag, with no package coupling.
type genericTestSlice []string

func (v *genericTestSlice) String() string     { return fmt.Sprint([]string(*v)) }
func (v *genericTestSlice) Type() string       { return "testSlice" }
func (v *genericTestSlice) Set(s string) error { return v.Append(s) }
func (v *genericTestSlice) Append(s string) error {
	*v = append(*v, s)
	return nil
}
func (v *genericTestSlice) Replace(s []string) error {
	*v = append(genericTestSlice(nil), s...)
	return nil
}
func (v *genericTestSlice) GetSlice() []string { return []string(*v) }

func TestGenericGoFlagInterop(t *testing.T) {
	fs := flag.NewFlagSet("generic", flag.ContinueOnError)
	var value int
	fs.NewVar(&value, "number", 0, "a number")
	boolean := newFlag(fs, "boolean", false, "a boolean")
	fs.New("slice", genericTestSlice(nil), "a custom slice")
	fs.New("native-slice", []int{1}, "a native slice")
	fs.New("map", map[string]int{"a": 1}, "a map")
	standard := goflag.NewFlagSet("standard", goflag.ContinueOnError)
	fs.CopyToGoFlagSet(standard)
	if err := standard.Parse([]string{"-number=42", "-boolean"}); err != nil {
		t.Fatal(err)
	}
	got, err := boolean.TryGet[bool]()
	if err != nil || !got || value != 42 {
		t.Fatalf("lost storage binding through standard flags: %d, %v, %v", value, got, err)
	}
	var help bytes.Buffer
	standard.SetOutput(&help)
	standard.PrintDefaults()
	if strings.Contains(help.String(), "panic") {
		t.Fatalf("standard help called String on a zero value:\n%s", help.String())
	}
}

func TestGenericCustomSlice(t *testing.T) {
	f := newFlag(newSet(), "custom", genericTestSlice{"default"}, "")
	slice, ok := f.Value.(flag.SliceValue)
	if !ok {
		t.Fatal("erasure discarded SliceValue")
	}
	if err := slice.Replace([]string{"replaced"}); err != nil {
		t.Fatal(err)
	}
	if err := slice.Append("appended"); err != nil {
		t.Fatal(err)
	}
	got, err := f.TryGet[genericTestSlice]()
	if err != nil || !reflect.DeepEqual(got, genericTestSlice{"replaced", "appended"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestGetPanicsWhereTryGetReturnsErrors(t *testing.T) {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	port := newFlag(fs, "port", uint16(8080), "")
	if got := port.Get[uint16](); got != 8080 {
		t.Fatalf("Flag.Get = %d", got)
	}
	if got := fs.Get[uint16]("port"); got != 8080 {
		t.Fatalf("FlagSet.Get = %d", got)
	}
	if got := flag.Get[uint16](port); got != 8080 {
		t.Fatalf("Get = %d", got)
	}

	for name, get := range map[string]func(){
		"wrong type":   func() { port.Get[int]() },
		"missing flag": func() { fs.Get[uint16]("absent") },
		"nil flag":     func() { flag.Get[uint16](nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Error("Get did not panic with an error")
				}
			}()
			get()
		})
	}

	var missing *flag.NotExistError
	if _, err := fs.TryGet[uint16]("absent"); !errors.As(err, &missing) {
		t.Fatalf("TryGet on a missing flag = %v", err)
	}
}
