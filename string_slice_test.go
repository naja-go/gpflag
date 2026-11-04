// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"strings"
	"testing"
)

func setUpSSFlagSet(ssp *[]string) (*FlagSet, *Flag[[]string]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar(ssp, "ss", []string{}, "Command separated list!")
	return f, flag
}

func setUpSSFlagSetWithDefault(ssp *[]string) (*FlagSet, *Flag[[]string]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar(ssp, "ss", []string{"default", "values"}, "Command separated list!")
	return f, flag
}

func TestEmptySS(t *testing.T) {
	var ss []string
	f, flag := setUpSSFlagSet(&ss)
	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err = f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSS := flag.Get()
	if len(getSS) != 0 {
		t.Fatalf("got ss %v with len=%d but expected length=0", getSS, len(getSS))
	}
}

func TestEmptySSValue(t *testing.T) {
	var ss []string
	f, flag := setUpSSFlagSet(&ss)
	err := f.Parse([]string{"--ss="})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err = f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSS := flag.Get()
	if len(getSS) != 0 {
		t.Fatalf("got ss %v with len=%d but expected length=0", getSS, len(getSS))
	}
}

func TestSS(t *testing.T) {
	var ss []string
	f, flag := setUpSSFlagSet(&ss)

	vals := []string{"one", "two", "4", "3"}
	arg := fmt.Sprintf("--ss=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range ss {
		if vals[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSS := flag.Get()
	for i, v := range getSS {
		if vals[i] != v {
			t.Fatalf("expected ss[%d] to be %s from GetStringSlice but got: %s", i, vals[i], v)
		}
	}
}

func TestSSDefault(t *testing.T) {
	var ss []string
	f, flag := setUpSSFlagSetWithDefault(&ss)

	vals := []string{"default", "values"}

	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range ss {
		if vals[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSS := flag.Get()
	for i, v := range getSS {
		if vals[i] != v {
			t.Fatalf("expected ss[%d] to be %s from GetStringSlice but got: %s", i, vals[i], v)
		}
	}
}

func TestSSWithDefault(t *testing.T) {
	var ss []string
	f, flag := setUpSSFlagSetWithDefault(&ss)

	vals := []string{"one", "two", "4", "3"}
	arg := fmt.Sprintf("--ss=%s", strings.Join(vals, ","))
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range ss {
		if vals[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSS := flag.Get()
	for i, v := range getSS {
		if vals[i] != v {
			t.Fatalf("expected ss[%d] to be %s from GetStringSlice but got: %s", i, vals[i], v)
		}
	}
}

func TestSSCalledTwice(t *testing.T) {
	var ss []string
	f, flag := setUpSSFlagSet(&ss)

	in := []string{"one,two", "three"}
	expected := []string{"one", "two", "three"}
	argfmt := "--ss=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(expected) != len(ss) {
		t.Fatalf("expected number of ss to be %d but got: %d", len(expected), len(ss))
	}
	for i, v := range ss {
		if expected[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	values := flag.Get()

	if len(expected) != len(values) {
		t.Fatalf("expected number of values to be %d but got: %d", len(expected), len(ss))
	}
	for i, v := range values {
		if expected[i] != v {
			t.Fatalf("expected got ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}

func TestSSWithComma(t *testing.T) {
	var ss []string
	f, flag := setUpSSFlagSet(&ss)

	in := []string{`"one,two"`, `"three"`, `"four,five",six`}
	expected := []string{"one,two", "three", "four,five", "six"}
	argfmt := "--ss=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	arg3 := fmt.Sprintf(argfmt, in[2])
	err := f.Parse([]string{arg1, arg2, arg3})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(expected) != len(ss) {
		t.Fatalf("expected number of ss to be %d but got: %d", len(expected), len(ss))
	}
	for i, v := range ss {
		if expected[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	values := flag.Get()

	if len(expected) != len(values) {
		t.Fatalf("expected number of values to be %d but got: %d", len(expected), len(values))
	}
	for i, v := range values {
		if expected[i] != v {
			t.Fatalf("expected got ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}

func TestSSWithSquareBrackets(t *testing.T) {
	var ss []string
	f, flag := setUpSSFlagSet(&ss)

	in := []string{`"[a-z]"`, `"[a-z]+"`}
	expected := []string{"[a-z]", "[a-z]+"}
	argfmt := "--ss=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(expected) != len(ss) {
		t.Fatalf("expected number of ss to be %d but got: %d", len(expected), len(ss))
	}
	for i, v := range ss {
		if expected[i] != v {
			t.Fatalf("expected ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	values := flag.Get()

	if len(expected) != len(values) {
		t.Fatalf("expected number of values to be %d but got: %d", len(expected), len(values))
	}
	for i, v := range values {
		if expected[i] != v {
			t.Fatalf("expected got ss[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}

func TestSSTypedReplacement(t *testing.T) {
	var ss []string
	f, _ := setUpSSFlagSet(&ss)

	in := []string{"one", "two"}
	argfmt := "--ss=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err := f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal(err)
	}
	flag.Set([]string{"three"})
	if len(ss) != 1 || ss[0] != "three" {
		t.Fatalf("Expected ss to be overwritten with 'three', but got: %s", ss)
	}
}

func TestSSTypedReplacementAppend(t *testing.T) {
	var ss []string
	f, _ := setUpSSFlagSet(&ss)
	err := f.Parse([]string{"--ss=one,two"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	flag, err := f.Lookup[[]string]("ss")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	flag.Set(append(flag.Get(), "three"))
	expected := []string{"one", "two", "three"}
	if len(ss) != len(expected) {
		t.Fatalf(
			"expected number of ss to be %d but got: %d",
			len(expected),
			len(ss),
		)
	}
	for i, v := range ss {
		if expected[i] != v {
			t.Fatalf(
				"expected ss[%d] to be %s but got: %s",
				i,
				expected[i],
				v,
			)
		}
	}
}
