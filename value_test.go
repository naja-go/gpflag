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
