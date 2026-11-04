// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"testing"
)

func setUpSAFlagSet(sap *[]string) (*FlagSet, *Flag[[]string]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar((*[]string)(sap), "sa", []string{}, "Command separated list!", StringArray())
	return f, flag
}

func setUpSAFlagSetWithDefault(sap *[]string) (*FlagSet, *Flag[[]string]) {
	f := NewFlagSet("test", ContinueOnError)
	flag := f.NewVar((*[]string)(sap), "sa", []string{"default", "values"}, "Command separated list!", StringArray())
	return f, flag
}

func TestEmptySA(t *testing.T) {
	var sa []string
	f, flag := setUpSAFlagSet(&sa)
	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err = f.Lookup[[]string]("sa")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSA := flag.Get()
	if len(getSA) != 0 {
		t.Fatalf("got sa %v with len=%d but expected length=0", getSA, len(getSA))
	}
}

func TestEmptySAValue(t *testing.T) {
	var sa []string
	f, flag := setUpSAFlagSet(&sa)
	err := f.Parse([]string{"--sa="})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err = f.Lookup[[]string]("sa")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSA := flag.Get()
	if len(getSA) != 1 || getSA[0] != "" {
		t.Fatalf("got sa %q with len=%d but expected one empty string", getSA, len(getSA))
	}
}

func TestSADefault(t *testing.T) {
	var sa []string
	f, flag := setUpSAFlagSetWithDefault(&sa)

	vals := []string{"default", "values"}

	err := f.Parse([]string{})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}
	for i, v := range sa {
		if vals[i] != v {
			t.Fatalf("expected sa[%d] to be %s but got: %s", i, vals[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("sa")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSA := flag.Get()
	for i, v := range getSA {
		if vals[i] != v {
			t.Fatalf("expected sa[%d] to be %s from GetStringArray but got: %s", i, vals[i], v)
		}
	}
}

func TestSAWithDefault(t *testing.T) {
	var sa []string
	f, flag := setUpSAFlagSetWithDefault(&sa)

	val := "one"
	arg := fmt.Sprintf("--sa=%s", val)
	err := f.Parse([]string{arg})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(sa) != 1 {
		t.Fatalf("expected number of values to be %d but %d", 1, len(sa))
	}

	if sa[0] != val {
		t.Fatalf("expected value to be %s but got: %s", sa[0], val)
	}

	flag, err = f.Lookup[[]string]("sa")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	getSA := flag.Get()

	if len(getSA) != 1 {
		t.Fatalf("expected number of values to be %d but %d", 1, len(getSA))
	}

	if getSA[0] != val {
		t.Fatalf("expected value to be %s but got: %s", getSA[0], val)
	}
}

func TestSACalledTwice(t *testing.T) {
	var sa []string
	f, flag := setUpSAFlagSet(&sa)

	in := []string{"one", "two"}
	expected := []string{"one", "two"}
	argfmt := "--sa=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(expected) != len(sa) {
		t.Fatalf("expected number of sa to be %d but got: %d", len(expected), len(sa))
	}
	for i, v := range sa {
		if expected[i] != v {
			t.Fatalf("expected sa[%d] to be %s but got: %s", i, expected[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("sa")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	values := flag.Get()

	if len(expected) != len(values) {
		t.Fatalf("expected number of values to be %d but got: %d", len(expected), len(sa))
	}
	for i, v := range values {
		if expected[i] != v {
			t.Fatalf("expected got sa[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}

func TestSAWithSpecialChar(t *testing.T) {
	var sa []string
	f, flag := setUpSAFlagSet(&sa)

	in := []string{"one,two", `"three"`, `"four,five",six`, "seven eight"}
	expected := []string{"one,two", `"three"`, `"four,five",six`, "seven eight"}
	argfmt := "--sa=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	arg3 := fmt.Sprintf(argfmt, in[2])
	arg4 := fmt.Sprintf(argfmt, in[3])
	err := f.Parse([]string{arg1, arg2, arg3, arg4})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(expected) != len(sa) {
		t.Fatalf("expected number of sa to be %d but got: %d", len(expected), len(sa))
	}
	for i, v := range sa {
		if expected[i] != v {
			t.Fatalf("expected sa[%d] to be %s but got: %s", i, expected[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("sa")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	values := flag.Get()

	if len(expected) != len(values) {
		t.Fatalf("expected number of values to be %d but got: %d", len(expected), len(values))
	}
	for i, v := range values {
		if expected[i] != v {
			t.Fatalf("expected got sa[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}

func TestSATypedReplacement(t *testing.T) {
	var sa []string
	f, _ := setUpSAFlagSet(&sa)

	in := []string{"1ns", "2ns"}
	argfmt := "--sa=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	err := f.Parse([]string{arg1, arg2})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	flag, err := f.Lookup[[]string]("sa")
	if err != nil {
		t.Fatal(err)
	}
	flag.Set([]string{"3ns"})
	if len(sa) != 1 || sa[0] != "3ns" {
		t.Fatalf("Expected ss to be overwritten with '3ns', but got: %v", sa)
	}
}

func TestSAWithSquareBrackets(t *testing.T) {
	var sa []string
	f, flag := setUpSAFlagSet(&sa)

	in := []string{"][]-[", "[a-z]", "[a-z]+"}
	expected := []string{"][]-[", "[a-z]", "[a-z]+"}
	argfmt := "--sa=%s"
	arg1 := fmt.Sprintf(argfmt, in[0])
	arg2 := fmt.Sprintf(argfmt, in[1])
	arg3 := fmt.Sprintf(argfmt, in[2])
	err := f.Parse([]string{arg1, arg2, arg3})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if len(expected) != len(sa) {
		t.Fatalf("expected number of sa to be %d but got: %d", len(expected), len(sa))
	}
	for i, v := range sa {
		if expected[i] != v {
			t.Fatalf("expected sa[%d] to be %s but got: %s", i, expected[i], v)
		}
	}

	flag, err = f.Lookup[[]string]("sa")
	if err != nil {
		t.Fatal("Lookup[[]string]:", err)
	}
	values := flag.Get()

	if len(expected) != len(values) {
		t.Fatalf("expected number of values to be %d but got: %d", len(expected), len(values))
	}
	for i, v := range values {
		if expected[i] != v {
			t.Fatalf("expected got sa[%d] to be %s but got: %s", i, expected[i], v)
		}
	}
}
