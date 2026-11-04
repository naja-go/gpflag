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

func setUpF64SFlagSet(f64sp *[]float64) (*FlagSet, *Flag[[]float64]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar(f64sp, "f64s", []float64{}, "Command separated list!")
	return f, flag
}

func setUpF64SFlagSetWithDefault(f64sp *[]float64) (*FlagSet, *Flag[[]float64]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar(f64sp, "f64s", []float64{0.0, 1.0}, "Command separated list!")
	return f, flag
}

func TestEmptyF64S(t *testing.T) {
	var f64s []float64
	f, flag := setUpF64SFlagSet(&f64s)
	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err = f.Lookup[[]float64]("f64s")
	if err != nil {
		t.Fatal("Lookup[[]float64]:", err)
	}
	getF64S := flag.Get()
	if len(getF64S) != 0 {
		t.Fatalf("got f64s %v with len=%d but expected length=0", getF64S, len(getF64S))
	}
}

func TestF64S(t *testing.T) {
	var f64s []float64
	f, flag := setUpF64SFlagSet(&f64s)

	vals := []string{"1.0", "2.0", "4.0", "3.0"}
	arg := fmt.Sprintf("--f64s=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range f64s {
		d, err := strconv.ParseFloat(vals[i], 64)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if d != v {
			t.Fatalf("expected f64s[%d] to be %s but got: %f", i, vals[i], v)
		}
	}
	flag, err = f.Lookup[[]float64]("f64s")
	if err != nil {
		t.Fatal("Lookup[[]float64]:", err)
	}
	getF64S := flag.Get()
	for i, v := range getF64S {
		d, err := strconv.ParseFloat(vals[i], 64)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if d != v {
			t.Fatalf("expected f64s[%d] to be %s but got: %f from GetFloat64Slice", i, vals[i], v)
		}
	}
}

func TestF64SDefault(t *testing.T) {
	var f64s []float64
	f, flag := setUpF64SFlagSetWithDefault(&f64s)

	vals := []string{"0.0", "1.0"}

	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range f64s {
		d, err := strconv.ParseFloat(vals[i], 64)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if d != v {
			t.Fatalf("expected f64s[%d] to be %f but got: %f", i, d, v)
		}
	}

	flag, err = f.Lookup[[]float64]("f64s")
	if err != nil {
		t.Fatal("Lookup[[]float64]:", err)
	}
	getF64S := flag.Get()
	for i, v := range getF64S {
		d, err := strconv.ParseFloat(vals[i], 64)
		if err != nil {
			t.Fatal("got an error from GetFloat64Slice():", err)
		}
		if d != v {
			t.Fatalf("expected f64s[%d] to be %f from GetFloat64Slice but got: %f", i, d, v)
		}
	}
}

func TestF64SWithDefault(t *testing.T) {
	var f64s []float64
	f, flag := setUpF64SFlagSetWithDefault(&f64s)

	vals := []string{"1.0", "2.0"}
	arg := fmt.Sprintf("--f64s=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range f64s {
		d, err := strconv.ParseFloat(vals[i], 64)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if d != v {
			t.Fatalf("expected f64s[%d] to be %f but got: %f", i, d, v)
		}
	}

	flag, err = f.Lookup[[]float64]("f64s")
	if err != nil {
		t.Fatal("Lookup[[]float64]:", err)
	}
	getF64S := flag.Get()
	for i, v := range getF64S {
		d, err := strconv.ParseFloat(vals[i], 64)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if d != v {
			t.Fatalf("expected f64s[%d] to be %f from GetFloat64Slice but got: %f", i, d, v)
		}
	}
}

func TestF64STypedReplacement(t *testing.T) {
	var f64s []float64
	f, _ := setUpF64SFlagSet(&f64s)

	in := []string{"1.0", "2.0"}
	argfmt := "--f64s=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err := f.Lookup[[]float64]("f64s")
	if err != nil {
		t.Fatal(err)
	}
	flag.Set([]float64{3.1})
	if len(f64s) != 1 || f64s[0] != 3.1 {
		t.Fatalf("Expected ss to be overwritten with '3.1', but got: %v", f64s)
	}
}

func TestF64SCalledTwice(t *testing.T) {
	var f64s []float64
	f, _ := setUpF64SFlagSet(&f64s)

	in := []string{"1.0,2.0", "3.0"}
	expected := []float64{1.0, 2.0, 3.0}
	argfmt := "--f64s=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range f64s {
		if expected[i] != v {
			t.Fatalf("expected f64s[%d] to be %f but got: %f", i, expected[i], v)
		}
	}
}
