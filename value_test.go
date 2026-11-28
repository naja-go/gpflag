// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"reflect"
	"testing"
)

type upperValue string

func (u *upperValue) String() string     { return string(*u) }
func (u *upperValue) Set(s string) error { *u = upperValue(s + "!"); return nil }
func (u *upperValue) Type() string       { return "upper" }

func TestCodecForUsesValueImplementations(t *testing.T) {
	c, ok := codecFor(reflect.TypeFor[upperValue]())
	if !ok || c.TypeName != "upper" {
		t.Fatalf("Value codec = %q, %v", c.TypeName, ok)
	}
	if v, _ := c.Parse("hi"); c.Format(v) != "hi!" {
		t.Errorf("Value codec round trip = %q", c.Format(v))
	}

	// The same codec makes a Value usable as a slice or map element.
	fs := NewFlagSet("test", ContinueOnError)
	fs.New("list", []upperValue{}, "")
	list := fs.Lookup("list")
	fs.New("dict", map[string]upperValue{}, "")
	dict := fs.Lookup("dict")
	if err := fs.Parse([]string{"--list=a,b", "--dict=k=v"}); err != nil {
		t.Fatal(err)
	}
	if got := list.Get[[]upperValue](); !reflect.DeepEqual(got, []upperValue{"a!", "b!"}) {
		t.Errorf("list = %q", got)
	}
	if got := dict.Get[map[string]upperValue](); !reflect.DeepEqual(got, map[string]upperValue{"k": "v!"}) {
		t.Errorf("dict = %q", got)
	}
	if list.Value.Type() != "upperSlice" || dict.Value.Type() != "stringToUpper" {
		t.Errorf("types = %q, %q", list.Value.Type(), dict.Value.Type())
	}
}

// levelValue has the Set and String methods of flag.Value, but no Type method.
type levelValue int

func (l *levelValue) String() string   { return "" }
func (l *levelValue) Set(string) error { return nil }

// textLevelValue also has UnmarshalText, so it is parsed as text.
type textLevelValue int

func (l *textLevelValue) String() string             { return "" }
func (l *textLevelValue) Set(string) error           { return nil }
func (l *textLevelValue) UnmarshalText([]byte) error { *l = 1; return nil }

func TestSetWithoutTypeIsRejected(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	mustPanic := func(name string, register func()) {
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected a panic", name)
			}
		}()
		register()
	}
	mustPanic("scalar", func() { fs.New("level", levelValue(0), "") })
	mustPanic("slice", func() { fs.New("levels", []levelValue{}, "") })

	level := fs.New("text-level", textLevelValue(0), "")
	if err := fs.Parse([]string{"--text-level=x"}); err != nil || *level != 1 {
		t.Errorf("text level = %d, err = %v", *level, err)
	}
}
