// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func setUpI32SFlagSet(isp *[]int32) (*FlagSet, *Flag[[]int32]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar(isp, "is", []int32{}, "Command separated list!")
	return f, flag
}

func setUpI32SFlagSetWithDefault(isp *[]int32) (*FlagSet, *Flag[[]int32]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar(isp, "is", []int32{0, 1}, "Command separated list!")
	return f, flag
}

func TestEmptyI32S(t *testing.T) {
	var is []int32
	f, flag := setUpI32SFlagSet(&is)
	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err = f.Lookup[[]int32]("is")
	if err != nil {
		t.Fatal("Lookup[[]int32]:", err)
	}
	getI32S := flag.Get()
	if len(getI32S) != 0 {
		t.Fatalf("got is %v with len=%d but expected length=0", getI32S, len(getI32S))
	}
}

func TestI32S(t *testing.T) {
	var is []int32
	f, flag := setUpI32SFlagSet(&is)

	vals := []string{"1", "2", "4", "3"}
	arg := fmt.Sprintf("--is=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range is {
		d64, err := strconv.ParseInt(vals[i], 0, 32)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		d := int32(d64)
		if d != v {
			t.Fatalf("expected is[%d] to be %s but got: %d", i, vals[i], v)
		}
	}
	flag, err = f.Lookup[[]int32]("is")
	if err != nil {
		t.Fatal("Lookup[[]int32]:", err)
	}
	getI32S := flag.Get()
	for i, v := range getI32S {
		d64, err := strconv.ParseInt(vals[i], 0, 32)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		d := int32(d64)
		if d != v {
			t.Fatalf("expected is[%d] to be %s but got: %d from GetInt32Slice", i, vals[i], v)
		}
	}
}

func TestI32SDefault(t *testing.T) {
	var is []int32
	f, flag := setUpI32SFlagSetWithDefault(&is)

	vals := []string{"0", "1"}

	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range is {
		d64, err := strconv.ParseInt(vals[i], 0, 32)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		d := int32(d64)
		if d != v {
			t.Fatalf("expected is[%d] to be %d but got: %d", i, d, v)
		}
	}

	flag, err = f.Lookup[[]int32]("is")
	if err != nil {
		t.Fatal("Lookup[[]int32]:", err)
	}
	getI32S := flag.Get()
	for i, v := range getI32S {
		d64, err := strconv.ParseInt(vals[i], 0, 32)
		if err != nil {
			t.Fatal("got an error from GetInt32Slice():", err)
		}
		d := int32(d64)
		if d != v {
			t.Fatalf("expected is[%d] to be %d from GetInt32Slice but got: %d", i, d, v)
		}
	}
}

func TestI32SWithDefault(t *testing.T) {
	var is []int32
	f, flag := setUpI32SFlagSetWithDefault(&is)

	vals := []string{"1", "2"}
	arg := fmt.Sprintf("--is=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range is {
		d64, err := strconv.ParseInt(vals[i], 0, 32)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		d := int32(d64)
		if d != v {
			t.Fatalf("expected is[%d] to be %d but got: %d", i, d, v)
		}
	}

	flag, err = f.Lookup[[]int32]("is")
	if err != nil {
		t.Fatal("Lookup[[]int32]:", err)
	}
	getI32S := flag.Get()
	for i, v := range getI32S {
		d64, err := strconv.ParseInt(vals[i], 0, 32)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		d := int32(d64)
		if d != v {
			t.Fatalf("expected is[%d] to be %d from GetInt32Slice but got: %d", i, d, v)
		}
	}
}

func TestI32STypedReplacement(t *testing.T) {
	var i32s []int32
	f, _ := setUpI32SFlagSet(&i32s)

	in := []string{"1", "2"}
	argfmt := "--is=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err := f.Lookup[[]int32]("is")
	if err != nil {
		t.Fatal(err)
	}
	flag.Set([]int32{3})
	if len(i32s) != 1 || i32s[0] != 3 {
		t.Fatalf("Expected ss to be overwritten with '3.1', but got: %v", i32s)
	}
}

func TestI32SCalledTwice(t *testing.T) {
	var is []int32
	f, _ := setUpI32SFlagSet(&is)

	in := []string{"1,2", "3"}
	expected := []int32{1, 2, 3}
	argfmt := "--is=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range is {
		if expected[i] != v {
			t.Fatalf("expected is[%d] to be %d but got: %d", i, expected[i], v)
		}
	}
}
