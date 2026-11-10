// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	flag "github.com/naja-go/gpflag"
	"github.com/naja-go/gpflag/pkg/types"
)

type unmarshalOnly string

func (u *unmarshalOnly) UnmarshalText(text []byte) error {
	*u = unmarshalOnly("parsed:" + string(text))
	return nil
}

func TestUnmarshalOnlyValue(t *testing.T) {
	f := newFlag(newSet(), "text", unmarshalOnly("default"), "")
	if err := f.Value.Set("hello"); err != nil {
		t.Fatal(err)
	}
	if got, err := f.TryGet[unmarshalOnly](); err != nil || got != "parsed:hello" {
		t.Fatalf("UnmarshalText was bypassed: %v, %v", got, err)
	}
	if f.Value.String() != "parsed:hello" {
		t.Fatalf("fallback formatting lost value: %s", f.Value.String())
	}
}

type firstAware []string

func (v *firstAware) Type() string       { return "firstAware" }
func (v *firstAware) String() string     { return fmt.Sprint([]string(*v)) }
func (v *firstAware) Set(s string) error { return v.SetFirst(s, false) }
func (v *firstAware) SetFirst(s string, first bool) error {
	if s == "bad" {
		return fmt.Errorf("bad value")
	}
	if first {
		*v = firstAware{s}
	} else {
		*v = append(*v, s)
	}
	return nil
}

func TestCustomFirstSuccessfulOccurrence(t *testing.T) {
	defaults := firstAware{"default"}
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	f := newFlag(fs, "value", defaults, "")
	if err := fs.Set("value", "bad"); err == nil || f.Changed {
		t.Fatal("failed first occurrence was accepted or marked changed")
	}
	if err := fs.Set("value", "first"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("value", "bad"); err == nil {
		t.Fatal("failed later occurrence was accepted")
	}
	if err := fs.Set("value", "second"); err != nil {
		t.Fatal(err)
	}
	if got, err := f.TryGet[firstAware](); err != nil || !reflect.DeepEqual(got, firstAware{"first", "second"}) {
		t.Fatalf("custom occurrence state = %v, %v", got, err)
	}
	if !reflect.DeepEqual(defaults, firstAware{"default"}) {
		t.Fatal("custom slice default was mutated")
	}
}

func TestEmptyStringCollectionIsNotZero(t *testing.T) {
	fs := flag.NewFlagSet("empty", flag.ContinueOnError)
	fs.New("slice", []string{""}, "")
	fs.New("array", types.StringArray{""}, "")
	usage := fs.FlagUsages()
	if strings.Count(usage, `(default [""])`) != 2 {
		t.Fatalf("single empty string was conflated with an empty collection:\n%s", usage)
	}
	if err := fs.Set("slice", `""`); err != nil {
		t.Fatal(err)
	}
	if got, err := fs.TryGet[[]string]("slice"); err != nil || !reflect.DeepEqual(got, []string{""}) {
		t.Fatalf("empty CSV element was lost: %#v, %v", got, err)
	}
}

func TestUnsupportedDefaultsDoNotChangeDestination(t *testing.T) {
	value := map[int]int{1: 2}
	defer func() {
		if recover() == nil {
			t.Error("unsupported map key did not panic")
		}
		if !reflect.DeepEqual(value, map[int]int{1: 2}) {
			t.Errorf("failed construction mutated the destination: %v", value)
		}
	}()
	newSet().NewVar(&value, "bad", map[int]int{3: 4}, "")
}

func TestHooksApplyHoweverAFlagIsRegistered(t *testing.T) {
	fs := flag.NewFlagSet("hooks", flag.ContinueOnError)
	var count types.Count
	items := types.StringArray{"default"}
	fs.AddFlag(&flag.Flag{Name: "verbose", Shorthand: "v", Value: &count, DefValue: count.String()})
	fs.AddFlag(&flag.Flag{Name: "item", Value: &items, DefValue: items.String()})

	if err := fs.Parse([]string{"-vvv", "--item=a", "--item=b"}); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("NoOptDefaulter was ignored by AddFlag: count = %d", count)
	}
	if !reflect.DeepEqual(items, types.StringArray{"a", "b"}) {
		t.Errorf("FirstSetter was ignored by AddFlag: items = %v", items)
	}
	if got := fs.Get[types.Count]("verbose"); got != 3 {
		t.Errorf("Get on a hand-built flag = %d", got)
	}
}

func TestNewKeepsCustomValueVisible(t *testing.T) {
	fs := flag.NewFlagSet("visible", flag.ContinueOnError)
	var count types.Count
	f := fs.NewVar(&count, "verbose", types.Count(1), "")
	if f.Value != flag.Value(&count) {
		t.Fatalf("flag.Value is %T, not the caller's *types.Count", f.Value)
	}
}

type level int

func (l *level) String() string     { return fmt.Sprint(int(*l)) }
func (l *level) Type() string       { return "level" }
func (l *level) Set(s string) error { _, err := fmt.Sscan(s, (*int)(l)); return err }

func TestNewAcceptsValuePointers(t *testing.T) {
	fs := flag.NewFlagSet("pointer", flag.ContinueOnError)
	lvl := level(1)
	f := newFlag(fs, "level", &lvl, "")
	if f.DefValue != "1" {
		t.Errorf("DefValue = %q", f.DefValue)
	}
	if err := fs.Parse([]string{"--level=4"}); err != nil {
		t.Fatal(err)
	}
	if lvl != 4 || f.Get[*level]() != &lvl || f.Get[level]() != 4 {
		t.Errorf("pointer Value was not used directly: %d", lvl)
	}

	var iface flag.Value = &lvl
	if g := newFlag(fs, "iface", iface, ""); g.Value != iface {
		t.Errorf("interface Value was wrapped: %T", g.Value)
	}

	defer func() {
		if recover() == nil {
			t.Error("nil Value pointer did not panic")
		}
	}()
	fs.New("nil", (*level)(nil), "")
}

type portNumber16 uint16

func TestNamedTypesReportTheirKind(t *testing.T) {
	fs := flag.NewFlagSet("kinds", flag.ContinueOnError)
	for name, f := range map[string]*flag.Flag{
		"uint16":         newFlag(fs, "port", portNumber16(1), ""),
		"uint16Slice":    newFlag(fs, "ports", []portNumber16{1}, ""),
		"stringToUint16": newFlag(fs, "limits", map[string]portNumber16{"a": 1}, ""),
	} {
		if f.Value.Type() != name {
			t.Errorf("--%s Type() = %q, want %q", f.Name, f.Value.Type(), name)
		}
	}
	if usage := fs.FlagUsages(); strings.Contains(usage, "portNumber16") {
		t.Errorf("Go type name leaked into help:\n%s", usage)
	}
	if got := fs.Get[portNumber16]("port"); got != 1 {
		t.Errorf("named type was lost by Get: %d", got)
	}
}

func TestOutOfRangeNumbersSayClearly(t *testing.T) {
	fs := flag.NewFlagSet("range", flag.ContinueOnError)
	fs.New("small", int8(0), "")
	fs.New("bytes", []byte{}, "")
	for arg, want := range map[string]string{
		"--small=300":   "out of range for int8",
		"--small=x":     "must be an integer",
		"--bytes=1,256": "out of range for uint8",
	} {
		if err := fs.Parse([]string{arg}); err == nil || !strings.HasSuffix(err.Error(), want) {
			t.Errorf("%s: error = %v, want suffix %q", arg, err, want)
		}
	}
}

func TestMapValuesTrimSpaceLikeSlices(t *testing.T) {
	fs := flag.NewFlagSet("trim", flag.ContinueOnError)
	limits := fs.New("limit", map[string]int{}, "")
	labels := fs.New("label", map[string]string{}, "")
	if err := fs.Parse([]string{"--limit=a=1 ,b= 2", "--label=a= x ,b=y"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*limits, map[string]int{"a": 1, "b": 2}) {
		t.Errorf("numeric map values were not trimmed: %v", *limits)
	}
	if !reflect.DeepEqual(*labels, map[string]string{"a": " x ", "b": "y"}) {
		t.Errorf("string map values were changed: %q", *labels)
	}
}

func TestNilBoolFuncIsAnError(t *testing.T) {
	fs := flag.NewFlagSet("nil", flag.ContinueOnError)
	fs.New("hook", types.BoolFunc(nil), "")
	if err := fs.Parse([]string{"--hook"}); err == nil {
		t.Fatal("nil BoolFunc was accepted")
	}
}

func TestTextTypesAreNamedText(t *testing.T) {
	fs := flag.NewFlagSet("text", flag.ContinueOnError)
	fs.New("one", unmarshalOnly(""), "")
	fs.New("many", []unmarshalOnly{}, "")
	fs.New("keyed", map[string]unmarshalOnly{}, "")
	for name, want := range map[string]string{"one": "text", "many": "textSlice", "keyed": "stringToText"} {
		if got := fs.Lookup(name).Value.Type(); got != want {
			t.Errorf("--%s Type() = %q, want %q", name, got, want)
		}
	}
}
