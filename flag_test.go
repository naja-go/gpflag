// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	_ = New[bool]("test_bool", false, "bool value").Ptr()
	_ = New[int]("test_int", 0, "int value").Ptr()
	_ = New[int64]("test_int64", 0, "int64 value").Ptr()
	_ = New[uint]("test_uint", 0, "uint value").Ptr()
	_ = New[uint64]("test_uint64", 0, "uint64 value").Ptr()
	_ = New[string]("test_string", "0", "string value").Ptr()
	_ = New[float64]("test_float64", 0, "float64 value").Ptr()
	_ = New[time.Duration]("test_duration", 0, "time.Duration value").Ptr()
	_ = New[int]("test_optional_int", 0, "optional int value").Ptr()

	normalizeFlagNameInvocations = 0
)

func boolString(s string) string {
	if s == "0" {
		return "false"
	}
	return "true"
}

func TestEverything(t *testing.T) {
	m := make(map[string]FlagMeta)
	desired := "0"
	visitor := func(f FlagMeta) {
		if len(f.Name) > 5 && f.Name[0:5] == "test_" {
			m[f.Name] = f
			ok := false
			switch {
			case CommandLine.formal[NormalizedName(f.Name)].text() == desired:
				ok = true
			case f.Name == "test_bool" && CommandLine.formal[NormalizedName(f.Name)].text() == boolString(desired):
				ok = true
			case f.Name == "test_duration" && CommandLine.formal[NormalizedName(f.Name)].text() == desired+"s":
				ok = true
			}
			if !ok {
				t.Error("Visit: bad value", CommandLine.formal[NormalizedName(f.Name)].text(), "for", f.Name)
			}
		}
	}
	VisitAll(visitor)
	if len(m) != 9 {
		t.Error("VisitAll misses some flags")
		for k, v := range m {
			t.Log(k, v)
		}
	}
	m = make(map[string]FlagMeta)
	Visit(visitor)
	if len(m) != 0 {
		t.Errorf("Visit sees unset flags")
		for k, v := range m {
			t.Log(k, v)
		}
	}
	// Now set all flags
	if err := SetText("test_bool", "true"); err != nil {
		t.Error(err)
	}
	if err := SetText("test_int", "1"); err != nil {
		t.Error(err)
	}
	if err := SetText("test_int64", "1"); err != nil {
		t.Error(err)
	}
	if err := SetText("test_uint", "1"); err != nil {
		t.Error(err)
	}
	if err := SetText("test_uint64", "1"); err != nil {
		t.Error(err)
	}
	if err := SetText("test_string", "1"); err != nil {
		t.Error(err)
	}
	if err := SetText("test_float64", "1"); err != nil {
		t.Error(err)
	}
	if err := SetText("test_duration", "1s"); err != nil {
		t.Error(err)
	}
	if err := SetText("test_optional_int", "1"); err != nil {
		t.Error(err)
	}
	desired = "1"
	Visit(visitor)
	if len(m) != 9 {
		t.Error("Visit fails after set")
		for k, v := range m {
			t.Log(k, v)
		}
	}
	// Now test they're visited in sort order.
	var flagNames []string
	Visit(func(f FlagMeta) { flagNames = append(flagNames, f.Name) })
	if !sort.StringsAreSorted(flagNames) {
		t.Errorf("flag names not sorted: %v", flagNames)
	}
}

func TestNoArgument(t *testing.T) {
	if GetCommandLine().Parse([]string{}) != nil {
		t.Error("parse failed for empty argument list")
	}
}

func TestUsage(t *testing.T) {
	called := false
	ResetForTesting(func() { called = true })
	err := GetCommandLine().Parse([]string{"--x"})
	expectedErr := "unknown flag: --x"
	if err == nil {
		t.Error("parse did not fail for unknown flag")
	}
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
	if called {
		t.Error("did call Usage while using ContinueOnError")
	}
}

func TestAddFlagSet(t *testing.T) {
	oldSet := NewFlagSet("old", ContinueOnError)
	newSet := NewFlagSet("new", ContinueOnError)

	oldSet.New[string]("flag1", "flag1", "flag1").Ptr()
	oldSet.New[string]("flag2", "flag2", "flag2").Ptr()

	newSet.New[string]("flag2", "flag2", "flag2").Ptr()
	newSet.New[string]("flag3", "flag3", "flag3").Ptr()

	oldSet.AddFlagSet(newSet)

	if len(oldSet.formal) != 3 {
		t.Errorf("Unexpected result adding a FlagSet to a FlagSet %v", oldSet)
	}
}

func TestAnnotation(t *testing.T) {
	f := NewFlagSet("shorthand", ContinueOnError)

	err := f.SetAnnotation("missing-flag", "key", nil)
	expectedErr := "no such flag -missing-flag"
	if err == nil {
		t.Errorf("Expected error setting annotation on non-existent flag")
	}
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}

	f.New[string]("stringa", "", "string value").Short('a').Ptr()
	if err := f.SetAnnotation("stringa", "key", nil); err != nil {
		t.Errorf("Unexpected error setting new nil annotation: %v", err)
	}
	if annotation := f.lookupCore("stringa").Annotations["key"]; annotation != nil {
		t.Errorf("Unexpected annotation: %v", annotation)
	}

	f.New[string]("stringb", "", "string2 value").Short('b').Ptr()
	if err := f.SetAnnotation("stringb", "key", []string{"value1"}); err != nil {
		t.Errorf("Unexpected error setting new annotation: %v", err)
	}
	if annotation := f.lookupCore("stringb").Annotations["key"]; !reflect.DeepEqual(annotation, []string{"value1"}) {
		t.Errorf("Unexpected annotation: %v", annotation)
	}

	if err := f.SetAnnotation("stringb", "key", []string{"value2"}); err != nil {
		t.Errorf("Unexpected error updating annotation: %v", err)
	}
	if annotation := f.lookupCore("stringb").Annotations["key"]; !reflect.DeepEqual(annotation, []string{"value2"}) {
		t.Errorf("Unexpected annotation: %v", annotation)
	}
}

func TestName(t *testing.T) {
	flagSetName := "bob"
	f := NewFlagSet(flagSetName, ContinueOnError)

	givenName := f.Name()
	if givenName != flagSetName {
		t.Errorf("Unexpected result when retrieving a FlagSet's name: expected %s, but found %s", flagSetName, givenName)
	}
}

func testParse(f *FlagSet, t *testing.T) {
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	boolFlag := f.New[bool]("bool", false, "bool value").Ptr()
	bool2Flag := f.New[bool]("bool2", false, "bool2 value").Ptr()
	bool3Flag := f.New[bool]("bool3", false, "bool3 value").Ptr()
	intFlag := f.New[int]("int", 0, "int value").Ptr()
	int8Flag := f.New[int8]("int8", 0, "int value").Ptr()
	int16Flag := f.New[int16]("int16", 0, "int value").Ptr()
	int32Flag := f.New[int32]("int32", 0, "int value").Ptr()
	int64Flag := f.New[int64]("int64", 0, "int64 value").Ptr()
	uintFlag := f.New[uint]("uint", 0, "uint value").Ptr()
	uint8Flag := f.New[uint8]("uint8", 0, "uint value").Ptr()
	uint16Flag := f.New[uint16]("uint16", 0, "uint value").Ptr()
	uint32Flag := f.New[uint32]("uint32", 0, "uint value").Ptr()
	uint64Flag := f.New[uint64]("uint64", 0, "uint64 value").Ptr()
	stringFlag := f.New[string]("string", "0", "string value").Ptr()
	float32Flag := f.New[float32]("float32", 0, "float32 value").Ptr()
	float64Flag := f.New[float64]("float64", 0, "float64 value").Ptr()
	ipFlag := f.New[net.IP]("ip", net.ParseIP("127.0.0.1"), "ip value").Ptr()
	maskFlag := f.New[net.IPMask]("mask", ParseIPv4Mask("0.0.0.0"), "mask value").Ptr()
	durationFlag := f.New[time.Duration]("duration", 5*time.Second, "time.Duration value").Ptr()
	optionalIntNoValueFlag := f.New[int]("optional-int-no-value", 0, "int value").SetNoOptDefVal("9").Ptr()
	optionalIntWithValueFlag := f.New[int]("optional-int-with-value", 0, "int value").Ptr()
	extra := "one-extra-argument"
	args := []string{
		"--bool",
		"--bool2=true",
		"--bool3=false",
		"--int=22",
		"--int8=-8",
		"--int16=-16",
		"--int32=-32",
		"--int64=0x23",
		"--uint", "24",
		"--uint8=8",
		"--uint16=16",
		"--uint32=32",
		"--uint64=25",
		"--string=hello",
		"--float32=-172e12",
		"--float64=2718e28",
		"--ip=10.11.12.13",
		"--mask=255.255.255.0",
		"--duration=2m",
		"--optional-int-no-value",
		"--optional-int-with-value=42",
		extra,
	}
	if err := f.Parse(args); err != nil {
		t.Fatal(err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	if *boolFlag != true {
		t.Error("bool flag should be true, is ", *boolFlag)
	}
	if *bool2Flag != true {
		t.Error("bool2 flag should be true, is ", *bool2Flag)
	}
	if *bool3Flag != false {
		t.Error("bool3 flag should be false, is ", *bool2Flag)
	}
	if *intFlag != 22 {
		t.Error("int flag should be 22, is ", *intFlag)
	}
	if *int8Flag != -8 {
		t.Error("int8 flag should be 0x23, is ", *int8Flag)
	}
	if *int16Flag != -16 {
		t.Error("int16 flag should be -16, is ", *int16Flag)
	}
	if *int32Flag != -32 {
		t.Error("int32 flag should be 0x23, is ", *int32Flag)
	}
	if *int64Flag != 0x23 {
		t.Error("int64 flag should be 0x23, is ", *int64Flag)
	}
	if *uintFlag != 24 {
		t.Error("uint flag should be 24, is ", *uintFlag)
	}
	if *uint8Flag != 8 {
		t.Error("uint8 flag should be 8, is ", *uint8Flag)
	}
	if *uint16Flag != 16 {
		t.Error("uint16 flag should be 16, is ", *uint16Flag)
	}
	if *uint32Flag != 32 {
		t.Error("uint32 flag should be 32, is ", *uint32Flag)
	}
	if *uint64Flag != 25 {
		t.Error("uint64 flag should be 25, is ", *uint64Flag)
	}
	if *stringFlag != "hello" {
		t.Error("string flag should be `hello`, is ", *stringFlag)
	}
	if *float32Flag != -172e12 {
		t.Error("float32 flag should be -172e12, is ", *float32Flag)
	}
	if *float64Flag != 2718e28 {
		t.Error("float64 flag should be 2718e28, is ", *float64Flag)
	}
	if !(*ipFlag).Equal(net.ParseIP("10.11.12.13")) {
		t.Error("ip flag should be 10.11.12.13, is ", *ipFlag)
	}
	if (*maskFlag).String() != ParseIPv4Mask("255.255.255.0").String() {
		t.Error("mask flag should be 255.255.255.0, is ", (*maskFlag).String())
	}
	if *durationFlag != 2*time.Minute {
		t.Error("duration flag should be 2m, is ", *durationFlag)
	}
	if *optionalIntNoValueFlag != 9 {
		t.Error("optional int flag should be the default value, is ", *optionalIntNoValueFlag)
	}
	if *optionalIntWithValueFlag != 42 {
		t.Error("optional int flag should be 42, is ", *optionalIntWithValueFlag)
	}
	if len(f.Args()) != 1 {
		t.Error("expected one argument, got", len(f.Args()))
	} else if f.Args()[0] != extra {
		t.Errorf("expected argument %q got %q", extra, f.Args()[0])
	}
	// Test unknown
	err := f.Parse([]string{"--unknown"})
	expectedErr := "unknown flag: --unknown"
	if err == nil {
		t.Error("parse did not fail for unknown flag")
	}
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
	// Test invalid
	err = f.Parse([]string{"--bool=abcdefg"})
	expectedErr = `invalid argument "abcdefg" for "--bool" flag: must be true or false`
	if err == nil {
		t.Error("parse did not fail for invalid argument")
	}
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
	// Test required
	err = f.Parse([]string{"--int"})
	expectedErr = `flag needs an argument: --int`
	if err == nil {
		t.Error("parse did not fail for missing argument")
	}
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func testParseAll(f *FlagSet, t *testing.T) {
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	f.New[bool]("boola", false, "bool value").Short('a').Ptr()
	f.New[bool]("boolb", false, "bool2 value").Short('b').Ptr()
	f.New[bool]("boolc", false, "bool3 value").Short('c').Ptr()
	f.New[bool]("boold", false, "bool4 value").Short('d').Ptr()
	f.New[string]("stringa", "0", "string value").Short('s').Ptr()
	f.New[string]("stringz", "0", "string value").Short('z').Ptr()
	f.New[string]("stringx", "0", "string value").Short('x').Ptr()
	f.New[string]("stringy", "0", "string value").Short('y').Ptr()
	f.lookupCore("stringx").NoOptDefVal = "1"
	args := []string{
		"-ab",
		"-cs=xx",
		"--stringz=something",
		"-d=true",
		"-x",
		"-y",
		"ee",
	}
	want := []string{
		"boola", "true",
		"boolb", "true",
		"boolc", "true",
		"stringa", "xx",
		"stringz", "something",
		"boold", "true",
		"stringx", "1",
		"stringy", "ee",
	}
	got := []string{}
	store := func(flag FlagMeta, value string) error {
		got = append(got, flag.Name)
		if len(value) > 0 {
			got = append(got, value)
		}
		return nil
	}
	if err := f.ParseAll(args, store); err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if !f.Parsed() {
		t.Errorf("f.Parse() = false after Parse")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("f.ParseAll() fail to restore the args")
		t.Errorf("Got:  %v", got)
		t.Errorf("Want: %v", want)
	}
}

func testParseWithUnknownFlags(f *FlagSet, t *testing.T, setUnknownFlags func(f *FlagSet)) {
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	setUnknownFlags(f)

	f.New[bool]("boola", false, "bool value").Short('a').Ptr()
	f.New[bool]("boolb", false, "bool2 value").Short('b').Ptr()
	f.New[bool]("boolc", false, "bool3 value").Short('c').Ptr()
	f.New[bool]("boold", false, "bool4 value").Short('d').Ptr()
	f.New[bool]("boole", false, "bool4 value").Short('e').Ptr()
	f.New[string]("stringa", "0", "string value").Short('s').Ptr()
	f.New[string]("stringz", "0", "string value").Short('z').Ptr()
	f.New[string]("stringx", "0", "string value").Short('x').Ptr()
	f.New[string]("stringy", "0", "string value").Short('y').Ptr()
	f.New[string]("stringo", "0", "string value").Short('o').Ptr()
	f.lookupCore("stringx").NoOptDefVal = "1"
	args := []string{
		"-ab",
		"-cs=xx",
		"--stringz=something",
		"--unknown1",
		"unknown1Value",
		"-d=true",
		"-x",
		"--unknown2=unknown2Value",
		"-u=unknown3Value",
		"-p",
		"unknown4Value",
		"-q", // another unknown with bool value
		"-y",
		"ee",
		"--unknown7=unknown7value",
		"--stringo=ovalue",
		"--unknown8=unknown8value",
		"--boole",
		"--unknown6",
		"",
		"-uuuuu",
		"",
		"--unknown10",
		"--unknown11",
	}
	want := []string{
		"boola", "true",
		"boolb", "true",
		"boolc", "true",
		"stringa", "xx",
		"stringz", "something",
		"boold", "true",
		"stringx", "1",
		"stringy", "ee",
		"stringo", "ovalue",
		"boole", "true",
	}
	got := []string{}
	store := func(flag FlagMeta, value string) error {
		got = append(got, flag.Name)
		if len(value) > 0 {
			got = append(got, value)
		}
		return nil
	}
	if err := f.ParseAll(args, store); err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if !f.Parsed() {
		t.Errorf("f.Parse() = false after Parse")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("f.ParseAll() fail to restore the args")
		t.Errorf("Got:  %v", got)
		t.Errorf("Want: %v", want)
	}
}

func testParseWithUnknownFlagsAndPassToArgs(f *FlagSet, t *testing.T, setUp ...func(*FlagSet)) {
	if f.Parsed() {
		t.Fatal("f.Parse() = true before Parse")
	}
	if len(setUp) == 0 {
		f.ParseErrorsAllowlist.UnknownFlagsHandling = PassUnknownFlagToArgs
	}
	for _, s := range setUp {
		s(f)
	}
	f.SetInterspersed(true)

	f.New[bool]("boola", false, "bool value").Short('a').Ptr()
	f.New[bool]("boolb", false, "bool2 value").Short('b').Ptr()
	f.New[bool]("boolc", false, "bool3 value").Short('c').Ptr()
	f.New[bool]("boold", false, "bool4 value").Short('d').Ptr()
	f.New[bool]("boole", false, "bool4 value").Short('e').Ptr()
	f.New[string]("stringa", "0", "string value").Short('s').Ptr()
	f.New[string]("stringz", "0", "string value").Short('z').Ptr()
	f.New[string]("stringx", "0", "string value").Short('x').Ptr()
	f.New[string]("stringy", "0", "string value").Short('y').Ptr()
	f.New[string]("stringo", "0", "string value").Short('o').Ptr()
	f.lookupCore("stringx").NoOptDefVal = "1"
	args := []string{
		"-ab",
		// -f and -g is unknown
		"-fcgs=xx",
		"--stringz=something",
		"--unknown1",
		"unknown1Value",
		"-d=true",
		"-x",
		"--unknown2=unknown2Value",
		"-u=unknown3Value",
		"-p",
		"unknown4Value",
		"-q", //another unknown with bool value
		"-y",
		"ee",
		"--unknown7=unknown7value",
		"--stringo=ovalue",
		"--unknown8=unknown8value",
		"--boole",
		"--unknown6",
		"",
		"-uuuuu",
		"",
		"--unknown10",
		"--unknown11",
		"arg0",
		"arg1",
	}
	want := []string{
		"boola", "true",
		"boolb", "true",
		"boolc", "true",
		"stringa", "xx",
		"stringz", "something",
		"boold", "true",
		"stringx", "1",
		"stringy", "ee",
		"stringo", "ovalue",
		"boole", "true",
	}
	wantArgs := []string{
		"-fg",
		"--unknown1",
		"unknown1Value",
		"--unknown2=unknown2Value",
		"-u=unknown3Value",
		"-p",
		"unknown4Value",
		"-q", //another unknown with bool value
		"--unknown7=unknown7value",
		"--unknown8=unknown8value",
		"--unknown6",
		"",
		"-uuuuu",
		"",
		"--unknown10",
		"--unknown11",
		"arg0",
		"arg1",
	}
	got := []string{}
	store := func(flag FlagMeta, value string) error {
		got = append(got, flag.Name)
		if len(value) > 0 {
			got = append(got, value)
		}
		return nil
	}
	if err := f.ParseAll(args, store); err != nil {
		t.Errorf("expected no error, got %s", err)
	}
	if !f.Parsed() {
		t.Errorf("f.Parse() = false after Parse")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("f.ParseAll() fail to restore the args")
		t.Errorf("Got:  %v", got)
		t.Errorf("Want: %v", want)
	}
	if !reflect.DeepEqual(f.Args(), wantArgs) {
		t.Errorf("f.ParseAll() fail to restore the args")
		t.Errorf("Got:  %v", f.Args())
		t.Errorf("Want: %v", wantArgs)
	}
}

func TestShorthand(t *testing.T) {
	f := NewFlagSet("shorthand", ContinueOnError)
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	boolaFlag := f.New[bool]("boola", false, "bool value").Short('a').Ptr()
	boolbFlag := f.New[bool]("boolb", false, "bool2 value").Short('b').Ptr()
	boolcFlag := f.New[bool]("boolc", false, "bool3 value").Short('c').Ptr()
	booldFlag := f.New[bool]("boold", false, "bool4 value").Short('d').Ptr()
	stringaFlag := f.New[string]("stringa", "0", "string value").Short('s').Ptr()
	stringzFlag := f.New[string]("stringz", "0", "string value").Short('z').Ptr()
	extra := "interspersed-argument"
	notaflag := "--i-look-like-a-flag"
	args := []string{
		"-ab",
		extra,
		"-cs",
		"hello",
		"-z=something",
		"-d=true",
		"--",
		notaflag,
	}
	f.SetOutput(ioutil.Discard)
	if err := f.Parse(args); err != nil {
		t.Error("expected no error, got ", err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	if *boolaFlag != true {
		t.Error("boola flag should be true, is ", *boolaFlag)
	}
	if *boolbFlag != true {
		t.Error("boolb flag should be true, is ", *boolbFlag)
	}
	if *boolcFlag != true {
		t.Error("boolc flag should be true, is ", *boolcFlag)
	}
	if *booldFlag != true {
		t.Error("boold flag should be true, is ", *booldFlag)
	}
	if *stringaFlag != "hello" {
		t.Error("stringa flag should be `hello`, is ", *stringaFlag)
	}
	if *stringzFlag != "something" {
		t.Error("stringz flag should be `something`, is ", *stringzFlag)
	}
	if len(f.Args()) != 2 {
		t.Error("expected one argument, got", len(f.Args()))
	} else if f.Args()[0] != extra {
		t.Errorf("expected argument %q got %q", extra, f.Args()[0])
	} else if f.Args()[1] != notaflag {
		t.Errorf("expected argument %q got %q", notaflag, f.Args()[1])
	}
	if f.ArgsLenAtDash() != 1 {
		t.Errorf("expected argsLenAtDash %d got %d", f.ArgsLenAtDash(), 1)
	}
	// Test unknown
	err := f.Parse([]string{"-ukn"})
	expectedErr := "unknown shorthand flag: 'u' in -ukn"
	if err == nil {
		t.Error("parse did not fail for unknown shorthand flag")
	}
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
	// Test required
	err = f.Parse([]string{"-as"})
	expectedErr = `flag needs an argument: 's' in -s`
	if err == nil {
		t.Error("parse did not fail for missing argument")
	}
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestShorthandLookup(t *testing.T) {
	f := NewFlagSet("shorthand", ContinueOnError)
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	f.New[bool]("boola", false, "bool value").Short('a').Ptr()
	f.New[bool]("boolb", false, "bool2 value").Short('b').Ptr()
	args := []string{
		"-ab",
	}
	f.SetOutput(ioutil.Discard)
	if err := f.Parse(args); err != nil {
		t.Error("expected no error, got ", err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	flag := f.ShorthandLookup("a")
	if flag == nil {
		t.Fatal("f.ShorthandLookup(\"a\") returned nil")
	}
	if flag.Name != "boola" {
		t.Errorf("f.ShorthandLookup(\"a\") found %q instead of \"boola\"", flag.Name)
	}
	flag = f.ShorthandLookup("")
	if flag != nil {
		t.Errorf("f.ShorthandLookup(\"\") did not return nil")
	}
	defer func() {
		_ = recover()
	}()
	_ = f.ShorthandLookup("ab")
	// should NEVER get here. lookup should panic. defer'd func should recover it.
	t.Errorf("f.ShorthandLookup(\"ab\") did not panic")
}

func TestParse(t *testing.T) {
	ResetForTesting(func() { t.Error("bad parse") })
	testParse(GetCommandLine(), t)
}

// TestInvalidArgumentMessages locks in the human-readable suffixes returned
// when typed flags fail to parse their input, replacing the prior raw stdlib
// errors (e.g. `strconv.ParseBool: parsing "x": invalid syntax`).
func TestInvalidArgumentMessages(t *testing.T) {
	cases := []struct {
		flag     string
		register func(*FlagSet)
		raw      string
		want     string
	}{
		{"bool", func(f *FlagSet) { f.New[bool]("bool", false, "").Ptr() }, "x",
			`invalid argument "x" for "--bool" flag: must be true or false`},
		{"int", func(f *FlagSet) { f.New[int]("int", 0, "").Ptr() }, "x",
			`invalid argument "x" for "--int" flag: must be an integer`},
		{"uint", func(f *FlagSet) { f.New[uint]("uint", 0, "").Ptr() }, "-1",
			`invalid argument "-1" for "--uint" flag: must be a non-negative integer`},
		{"float64", func(f *FlagSet) { f.New[float64]("float64", 0, "").Ptr() }, "x",
			`invalid argument "x" for "--float64" flag: must be a number`},
		{"duration", func(f *FlagSet) { f.New[time.Duration]("duration", 0, "").Ptr() }, "soon",
			`invalid argument "soon" for "--duration" flag: must be a duration like "30s" or "5m"`},
	}
	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			f := NewFlagSet("test", ContinueOnError)
			f.SetOutput(ioutil.Discard)
			tc.register(f)
			err := f.Parse([]string{"--" + tc.flag + "=" + tc.raw})
			if err == nil {
				t.Fatalf("expected error parsing --%s=%q, got nil", tc.flag, tc.raw)
			}
			if err.Error() != tc.want {
				t.Errorf("expected %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestParseAll(t *testing.T) {
	ResetForTesting(func() { t.Error("bad parse") })
	testParseAll(GetCommandLine(), t)
}

func TestIgnoreUnknownFlags(t *testing.T) {
	ResetForTesting(func() { t.Error("bad parse") })
	testParseWithUnknownFlags(GetCommandLine(), t, func(f *FlagSet) { f.ParseErrorsAllowlist.UnknownFlags = true })
}

func TestIgnoreUnknownFlagsBackwardsCompat(t *testing.T) {
	ResetForTesting(func() { t.Error("bad parse") })
	testParseWithUnknownFlags(GetCommandLine(), t, func(f *FlagSet) { f.ParseErrorsWhitelist.UnknownFlags = true })
}

func TestIgnoreUnknownFlagsAndPassToArgs(t *testing.T) {
	ResetForTesting(func() { t.Error("bad parse") })
	testParseWithUnknownFlagsAndPassToArgs(GetCommandLine(), t)
}

func TestIgnoreUnknownFlagsAndPassToArgsBackwardsCompat(t *testing.T) {
	ResetForTesting(func() { t.Error("bad parse") })
	testParseWithUnknownFlagsAndPassToArgs(GetCommandLine(), t, func(f *FlagSet) {
		f.ParseErrorsWhitelist.UnknownFlagsHandling = PassUnknownFlagToArgs
	})
}
func TestFlagSetParse(t *testing.T) {
	testParse(NewFlagSet("test", ContinueOnError), t)
}

func TestParseRepeated(t *testing.T) {
	fs := NewFlagSet("test repeated", ContinueOnError)

	t.Run("first parse", func(t *testing.T) {
		err := fs.Parse([]string{"foo", "bar"})
		if err != nil {
			t.Fatal("expected no error, got ", err)
		}

		argsAfterFirst := fs.Args()
		if !reflect.DeepEqual(argsAfterFirst, []string{"foo", "bar"}) {
			t.Fatalf("expected args [foo bar], got %v", argsAfterFirst)
		}
	})

	t.Run("re-parse with fewer args", func(t *testing.T) {
		err := fs.Parse([]string{"baz"})
		if err != nil {
			t.Fatal("expected no error, got ", err)
		}

		argsAfterSecond := fs.Args()
		if !reflect.DeepEqual(argsAfterSecond, []string{"baz"}) {
			t.Fatalf("expected args [baz], got %v", argsAfterSecond)
		}
	})

	t.Run("re-parse with no args", func(t *testing.T) {
		err := fs.Parse([]string{})
		if err != nil {
			t.Fatal("expected no error, got ", err)
		}

		argsAfterThird := fs.Args()
		if !reflect.DeepEqual(argsAfterThird, []string{}) {
			t.Fatalf("expected args [], got %v", argsAfterThird)
		}
	})
}

func TestChangedHelper(t *testing.T) {
	f := NewFlagSet("changedtest", ContinueOnError)
	f.New[bool]("changed", false, "changed bool").Ptr()
	f.New[bool]("settrue", true, "true to true").Ptr()
	f.New[bool]("setfalse", false, "false to false").Ptr()
	f.New[bool]("unchanged", false, "unchanged bool").Ptr()

	args := []string{"--changed", "--settrue", "--setfalse=false"}
	if err := f.Parse(args); err != nil {
		t.Error("f.Parse() = false after Parse")
	}
	if !f.Changed("changed") {
		t.Errorf("--changed wasn't changed!")
	}
	if !f.Changed("settrue") {
		t.Errorf("--settrue wasn't changed!")
	}
	if !f.Changed("setfalse") {
		t.Errorf("--setfalse wasn't changed!")
	}
	if f.Changed("unchanged") {
		t.Errorf("--unchanged was changed!")
	}
	if f.Changed("invalid") {
		t.Errorf("--invalid was changed!")
	}
	if f.ArgsLenAtDash() != -1 {
		t.Errorf("Expected argsLenAtDash: %d but got %d", -1, f.ArgsLenAtDash())
	}
}

func toDotSeparated(name string, from []string) string {
	result := name
	for _, sep := range from {
		result = strings.ReplaceAll(result, sep, ".")
	}
	// Type convert to indicate normalization has been done.
	return result
}

func wordSepNormalizeFunc(_ *FlagSet, name string) NormalizedName {
	seps := []string{"-", "_"}
	name = toDotSeparated(name, seps)
	normalizeFlagNameInvocations++

	return NormalizedName(name)
}

func testWordSepNormalizedNames(args []string, t *testing.T) {
	f := NewFlagSet("normalized", ContinueOnError)
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	withDashFlag := f.New[bool]("with-dash-flag", false, "bool value").Ptr()
	// Set this after some flags have been added and before others.
	f.SetNormalizeFunc(wordSepNormalizeFunc)
	withUnderFlag := f.New[bool]("with_under_flag", false, "bool value").Ptr()
	withBothFlag := f.New[bool]("with-both_flag", false, "bool value").Ptr()
	if err := f.Parse(args); err != nil {
		t.Fatal(err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	if *withDashFlag != true {
		t.Error("withDashFlag flag should be true, is ", *withDashFlag)
	}
	if *withUnderFlag != true {
		t.Error("withUnderFlag flag should be true, is ", *withUnderFlag)
	}
	if *withBothFlag != true {
		t.Error("withBothFlag flag should be true, is ", *withBothFlag)
	}
}

func TestWordSepNormalizedNames(t *testing.T) {
	args := []string{
		"--with-dash-flag",
		"--with-under-flag",
		"--with-both-flag",
	}
	testWordSepNormalizedNames(args, t)

	args = []string{
		"--with_dash_flag",
		"--with_under_flag",
		"--with_both_flag",
	}
	testWordSepNormalizedNames(args, t)

	args = []string{
		"--with-dash_flag",
		"--with-under_flag",
		"--with-both_flag",
	}
	testWordSepNormalizedNames(args, t)
}

func aliasAndWordSepFlagNames(_ *FlagSet, name string) NormalizedName {
	seps := []string{"-", "_"}

	oldName := toDotSeparated("old-valid_flag", seps)
	newName := toDotSeparated("valid-flag", seps)

	name = toDotSeparated(name, seps)
	switch name {
	case oldName:
		name = newName
	}

	return NormalizedName(name)
}

func TestCustomNormalizedNames(t *testing.T) {
	f := NewFlagSet("normalized", ContinueOnError)
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}

	validFlag := f.New[bool]("valid-flag", false, "bool value").Ptr()
	f.SetNormalizeFunc(aliasAndWordSepFlagNames)
	someOtherFlag := f.New[bool]("some-other-flag", false, "bool value").Ptr()

	args := []string{"--old_valid_flag", "--some-other_flag"}
	if err := f.Parse(args); err != nil {
		t.Fatal(err)
	}

	if *validFlag != true {
		t.Errorf("validFlag is %v even though we set the alias --old_valid_falg", *validFlag)
	}
	if *someOtherFlag != true {
		t.Error("someOtherFlag should be true, is ", *someOtherFlag)
	}
}

// Every flag we add, the name (displayed also in usage) should normalized
func TestNormalizationFuncShouldChangeFlagName(t *testing.T) {
	// Test normalization after addition
	f := NewFlagSet("normalized", ContinueOnError)

	f.New[bool]("valid_flag", false, "bool value").Ptr()
	if f.lookupCore("valid_flag").Name != "valid_flag" {
		t.Error("The new flag should have the name 'valid_flag' instead of ", f.lookupCore("valid_flag").Name)
	}

	f.SetNormalizeFunc(wordSepNormalizeFunc)
	if f.lookupCore("valid_flag").Name != "valid.flag" {
		t.Error("The new flag should have the name 'valid.flag' instead of ", f.lookupCore("valid_flag").Name)
	}

	// Test normalization before addition
	f = NewFlagSet("normalized", ContinueOnError)
	f.SetNormalizeFunc(wordSepNormalizeFunc)

	f.New[bool]("valid_flag", false, "bool value").Ptr()
	if f.lookupCore("valid_flag").Name != "valid.flag" {
		t.Error("The new flag should have the name 'valid.flag' instead of ", f.lookupCore("valid_flag").Name)
	}
}

// Related to https://github.com/spf13/cobra/issues/521.
func TestNormalizationSharedFlags(t *testing.T) {
	f := NewFlagSet("set f", ContinueOnError)
	g := NewFlagSet("set g", ContinueOnError)
	nfunc := wordSepNormalizeFunc
	testName := "valid_flag"
	normName := nfunc(nil, testName)
	if testName == string(normName) {
		t.Error("TestNormalizationSharedFlags meaningless: the original and normalized flag names are identical:", testName)
	}

	f.New[bool](testName, false, "bool value").Ptr()
	g.AddFlagSet(f)

	f.SetNormalizeFunc(nfunc)
	g.SetNormalizeFunc(nfunc)

	if len(f.formal) != 1 {
		t.Error("Normalizing flags should not result in duplications in the flag set:", f.formal)
	}
	if f.orderedFormal[0].Name != string(normName) {
		t.Error("Flag name not normalized")
	}
	for k := range f.formal {
		if k != "valid.flag" {
			t.Errorf("The key in the flag map should have been normalized: wanted \"%s\", got \"%s\" instead", normName, k)
		}
	}

	if f.lookupCore("valid.flag").Name != g.lookupCore("valid.flag").Name || f.formal["valid.flag"] == g.formal["valid.flag"] {
		t.Error("Two flag sets sharing the same flags should stay consistent after being normalized. Original set:", f.formal, "Duplicate set:", g.formal)
	}
}

func TestNormalizationSetFlags(t *testing.T) {
	f := NewFlagSet("normalized", ContinueOnError)
	nfunc := wordSepNormalizeFunc
	testName := "valid_flag"
	normName := nfunc(nil, testName)
	if testName == string(normName) {
		t.Error("TestNormalizationSetFlags meaningless: the original and normalized flag names are identical:", testName)
	}

	f.New[bool](testName, false, "bool value").Ptr()
	if err := f.SetText(testName, "true"); err != nil {
		t.Error(err)
	}
	f.SetNormalizeFunc(nfunc)

	if len(f.formal) != 1 {
		t.Error("Normalizing flags should not result in duplications in the flag set:", f.formal)
	}
	if f.orderedFormal[0].Name != string(normName) {
		t.Error("Flag name not normalized")
	}
	for k := range f.formal {
		if k != "valid.flag" {
			t.Errorf("The key in the flag map should have been normalized: wanted \"%s\", got \"%s\" instead", normName, k)
		}
	}

	if !reflect.DeepEqual(f.formal, f.actual) {
		t.Error("The map of set flags should get normalized. Formal:", f.formal, "Actual:", f.actual)
	}
}

// Declare a user-defined flag type.
type flagVar []string

func (f *flagVar) String() string {
	return fmt.Sprint([]string(*f))
}

func (f *flagVar) UnmarshalText(text []byte) error {
	value := string(text)
	*f = append(*f, value)
	return nil
}

func (f flagVar) MarshalText() ([]byte, error) { return []byte(fmt.Sprint([]string(f))), nil }

func TestUserDefined(t *testing.T) {
	var flags FlagSet
	flags.Init("test", ContinueOnError)
	var v flagVar
	flags.NewVarP(&v, "v", "v", v, "usage")
	if err := flags.Parse([]string{"--v=1", "-v2", "-v", "3"}); err != nil {
		t.Error(err)
	}
	if len(v) != 3 {
		t.Fatal("expected 3 args; got ", len(v))
	}
	expect := "[1 2 3]"
	if v.String() != expect {
		t.Errorf("expected value %q got %q", expect, v.String())
	}
}

func TestSetOutput(t *testing.T) {
	var flags FlagSet
	var buf bytes.Buffer
	flags.SetOutput(&buf)
	flags.Init("test", ContinueOnError)

	expectedErr := "unknown flag: --unknown"
	err := flags.Parse([]string{"--unknown"})
	if err == nil || err.Error() != expectedErr {
		t.Errorf("expected error %q got %v", expectedErr, err)
	}
	if out := buf.String(); !strings.Contains(out, "--unknown") {
		t.Logf("expected output mentioning unknown; got %q", out)
	}
}

func TestOutput(t *testing.T) {
	var flags FlagSet
	var buf bytes.Buffer
	expect := "an example string"
	flags.SetOutput(&buf)
	_, _ = fmt.Fprint(flags.Output(), expect)
	if out := buf.String(); !strings.Contains(out, expect) {
		t.Errorf("expected output %q; got %q", expect, out)
	}
}

// This tests that one can reset the flags. This still works but not well, and is
// superseded by FlagSet.
func TestChangingArgs(t *testing.T) {
	ResetForTesting(func() { t.Fatal("bad parse") })
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd", "--before", "subcmd"}
	before := New[bool]("before", false, "").Ptr()
	if err := GetCommandLine().Parse(os.Args[1:]); err != nil {
		t.Fatal(err)
	}
	cmd := Arg(0)
	os.Args = []string{"subcmd", "--after", "args"}
	after := New[bool]("after", false, "").Ptr()
	Parse()
	args := Args()

	if !*before || cmd != "subcmd" || !*after || len(args) != 1 || args[0] != "args" {
		t.Fatalf("expected true subcmd true [args] got %v %v %v %v", *before, cmd, *after, args)
	}
}

// Test that -help invokes the usage message and returns ErrHelp.
func TestHelp(t *testing.T) {
	helpCalled := false
	fs := NewFlagSet("help test", ContinueOnError)
	fs.Usage = func() { helpCalled = true }
	var flag bool
	fs.NewVar(&flag, "flag", false, "regular flag")
	// Regular flag invocation should work
	err := fs.Parse([]string{"--flag=true"})
	if err != nil {
		t.Fatal("expected no error; got ", err)
	}
	if !flag {
		t.Error("flag was not set by --flag")
	}
	if helpCalled {
		t.Error("help called for regular flag")
		helpCalled = false // reset for next test
	}
	// Help flag should work as expected.
	err = fs.Parse([]string{"--help"})
	if err == nil {
		t.Fatal("error expected")
	}
	if err != ErrHelp { //nolint:errorlint // not using errors.Is for compatibility with go1.12
		t.Fatal("expected ErrHelp; got ", err)
	}
	if !helpCalled {
		t.Fatal("help was not called")
	}
	// If we define a help flag, that should override.
	var help bool
	fs.NewVar(&help, "help", false, "help flag")
	helpCalled = false
	err = fs.Parse([]string{"--help"})
	if err != nil {
		t.Fatal("expected no error for defined --help; got ", err)
	}
	if helpCalled {
		t.Fatal("help was called; should not have been for defined help flag")
	}
}

func TestNoInterspersed(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	f.SetInterspersed(false)
	f.New[bool]("true", true, "always true").Ptr()
	f.New[bool]("false", false, "always false").Ptr()
	err := f.Parse([]string{"--true", "break", "--false"})
	if err != nil {
		t.Fatal("expected no error; got ", err)
	}
	args := f.Args()
	if len(args) != 2 || args[0] != "break" || args[1] != "--false" {
		t.Fatal("expected interspersed options/non-options to fail")
	}
}

func TestTermination(t *testing.T) {
	f := NewFlagSet("termination", ContinueOnError)
	boolFlag := f.New[bool]("bool", false, "bool value").Short('l').Ptr()
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	arg1 := "ls"
	arg2 := "-l"
	args := []string{
		"--",
		arg1,
		arg2,
	}
	f.SetOutput(ioutil.Discard)
	if err := f.Parse(args); err != nil {
		t.Fatal("expected no error; got ", err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	if *boolFlag {
		t.Error("expected boolFlag=false, got true")
	}
	if len(f.Args()) != 2 {
		t.Errorf("expected 2 arguments, got %d: %v", len(f.Args()), f.Args())
	}
	if f.Args()[0] != arg1 {
		t.Errorf("expected argument %q got %q", arg1, f.Args()[0])
	}
	if f.Args()[1] != arg2 {
		t.Errorf("expected argument %q got %q", arg2, f.Args()[1])
	}
	if f.ArgsLenAtDash() != 0 {
		t.Errorf("expected argsLenAtDash %d got %d", 0, f.ArgsLenAtDash())
	}
}

func getDeprecatedFlagSet() *FlagSet {
	f := NewFlagSet("bob", ContinueOnError)
	f.New[bool]("badflag", true, "always true").Ptr()
	_ = f.MarkDeprecated("badflag", "use --good-flag instead")
	return f
}

func TestDeprecatedFlagInDocs(t *testing.T) {
	f := getDeprecatedFlagSet()

	out := new(bytes.Buffer)
	f.SetOutput(out)
	f.PrintDefaults()

	if strings.Contains(out.String(), "badflag") {
		t.Errorf("found deprecated flag in usage!")
	}
}

func TestUnHiddenDeprecatedFlagInDocs(t *testing.T) {
	f := getDeprecatedFlagSet()
	flg := f.lookupCore("badflag")
	if flg == nil {
		t.Fatalf("Unable to lookup 'bob' in TestUnHiddenDeprecatedFlagInDocs")
	}
	flg.Hidden = false

	out := new(bytes.Buffer)
	f.SetOutput(out)
	f.PrintDefaults()

	defaults := out.String()
	if !strings.Contains(defaults, "badflag") {
		t.Errorf("Did not find deprecated flag in usage!")
	}
	if !strings.Contains(defaults, "use --good-flag instead") {
		t.Errorf("Did not find 'use --good-flag instead' in defaults")
	}
}

func TestDeprecatedFlagShorthandInDocs(t *testing.T) {
	f := NewFlagSet("bob", ContinueOnError)
	name := "noshorthandflag"
	f.New[bool](name, true, "always true").Short('n').Ptr()
	_ = f.MarkShorthandDeprecated("noshorthandflag", fmt.Sprintf("use --%s instead", name))

	out := new(bytes.Buffer)
	f.SetOutput(out)
	f.PrintDefaults()

	if strings.Contains(out.String(), "-n,") {
		t.Errorf("found deprecated flag shorthand in usage!")
	}
}

func parseReturnStderr(t *testing.T, f *FlagSet, args []string) (string, error) {
	t.Helper()
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := f.Parse(args)

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outC <- buf.String()
	}()

	_ = w.Close()
	os.Stderr = oldStderr
	out := <-outC

	return out, err
}

func TestDeprecatedFlagUsage(t *testing.T) {
	f := NewFlagSet("bob", ContinueOnError)
	f.New[bool]("badflag", true, "always true").Ptr()
	usageMsg := "use --good-flag instead"
	_ = f.MarkDeprecated("badflag", usageMsg)

	args := []string{"--badflag"}
	out, err := parseReturnStderr(t, f, args)
	if err != nil {
		t.Fatal("expected no error; got ", err)
	}

	if !strings.Contains(out, usageMsg) {
		t.Errorf("usageMsg not printed when using a deprecated flag!")
	}
}

func TestDeprecatedFlagShorthandUsage(t *testing.T) {
	f := NewFlagSet("bob", ContinueOnError)
	name := "noshorthandflag"
	f.New[bool](name, true, "always true").Short('n').Ptr()
	usageMsg := fmt.Sprintf("use --%s instead", name)
	_ = f.MarkShorthandDeprecated(name, usageMsg)

	args := []string{"-n"}
	out, err := parseReturnStderr(t, f, args)
	if err != nil {
		t.Fatal("expected no error; got ", err)
	}

	if !strings.Contains(out, usageMsg) {
		t.Errorf("usageMsg not printed when using a deprecated flag!")
	}
}

func TestDeprecatedFlagUsageNormalized(t *testing.T) {
	f := NewFlagSet("bob", ContinueOnError)
	f.New[bool]("bad-double_flag", true, "always true").Ptr()
	f.SetNormalizeFunc(wordSepNormalizeFunc)
	usageMsg := "use --good-flag instead"
	_ = f.MarkDeprecated("bad_double-flag", usageMsg)

	args := []string{"--bad_double_flag"}
	out, err := parseReturnStderr(t, f, args)
	if err != nil {
		t.Fatal("expected no error; got ", err)
	}

	if !strings.Contains(out, usageMsg) {
		t.Errorf("usageMsg not printed when using a deprecated flag!")
	}
}

func TestDeprecatedFlagUsageAlias(t *testing.T) {
	// When a NormalizeFunc aliases one flag name to another, using the alias
	// to set a deprecated flag should report the name the user actually typed,
	// not the canonical name. See https://github.com/naja-go/gpflag/issues/279.
	f := NewFlagSet("bob", ContinueOnError)
	f.New[string]("src", "", "src directory or file").Ptr()
	f.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
		if name == "dir" {
			name = "src"
		}
		return NormalizedName(name)
	})
	_ = f.MarkDeprecated("src", "use --dest")

	out, err := parseReturnStderr(t, f, []string{"--dir=x"})
	if err != nil {
		t.Fatal("expected no error; got ", err)
	}

	if !strings.Contains(out, "Flag --dir has been deprecated") {
		t.Errorf("expected the deprecation message to name the flag the user typed (--dir); got: %q", out)
	}
	if strings.Contains(out, "Flag --src has been deprecated") {
		t.Errorf("deprecation message should not name the canonical flag (--src); got: %q", out)
	}
}

// Name normalization function should be called only once on flag addition
func TestMultipleNormalizeFlagNameInvocations(t *testing.T) {
	normalizeFlagNameInvocations = 0

	f := NewFlagSet("normalized", ContinueOnError)
	f.SetNormalizeFunc(wordSepNormalizeFunc)
	f.New[bool]("with_under_flag", false, "bool value").Ptr()

	if normalizeFlagNameInvocations != 1 {
		t.Fatal("Expected normalizeFlagNameInvocations to be 1; got ", normalizeFlagNameInvocations)
	}
}

func TestHiddenFlagInUsage(t *testing.T) {
	f := NewFlagSet("bob", ContinueOnError)
	f.New[bool]("secretFlag", true, "shhh").Ptr()
	_ = f.MarkHidden("secretFlag")

	out := new(bytes.Buffer)
	f.SetOutput(out)
	f.PrintDefaults()

	if strings.Contains(out.String(), "secretFlag") {
		t.Errorf("found hidden flag in usage!")
	}
}

func TestHiddenFlagUsage(t *testing.T) {
	f := NewFlagSet("bob", ContinueOnError)
	f.New[bool]("secretFlag", true, "shhh").Ptr()
	_ = f.MarkHidden("secretFlag")

	args := []string{"--secretFlag"}
	out, err := parseReturnStderr(t, f, args)
	if err != nil {
		t.Fatal("expected no error; got ", err)
	}

	if strings.Contains(out, "shhh") {
		t.Errorf("usage message printed when using a hidden flag!")
	}
}

const defaultOutput = `      --A[=true|false]               for bootstrapping, allow 'any' type
      --Alongflagname[=true|false]   disable bounds checking
  -C, --CCC[=true|false]             a boolean defaulting to true (default true)
      --D path                       set relative path for local imports
  -E, --EEE num[=1234]               a num with NoOptDefVal (default 4321)
      --F number                     a non-zero number (default 2.7)
      --G float                      a float that defaults to zero
      --IP ip                        IP address with no default
      --IPMask ipMask                Netmask address with no default
      --IPNet ipNet                  IP network with no default
      --Ints ints                    int slice with zero default
      --N int                        a non-zero int (default 27)
      --ND1 string[="bar"]           a string with NoOptDefVal (default "foo")
      --ND2 num[=4321]               a num with NoOptDefVal (default 1234)
      --StringArray stringArray      string array with zero default
      --StringSlice strings          string slice with zero default
      --Z int                        an int that defaults to zero
      --custom custom                custom Value implementation
      --custom-with-val custom       custom value which has been set from command line while help is shown
      --customP default              a VarP with default (default 10)
      --maxT timeout                 set timeout for dial
  -v, --verbose count                verbosity
`

// Custom value that satisfies the Value interface.
type customValue int

func (cv *customValue) String() string { return fmt.Sprintf("%v", *cv) }

func (cv *customValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	*cv = customValue(v)
	return err
}

func (cv *customValue) UnmarshalText(text []byte) error { return cv.Set(string(text)) }
func (cv customValue) MarshalText() ([]byte, error)     { return []byte(fmt.Sprint(int(cv))), nil }

type customBoolFlagValue bool

func (cv *customBoolFlagValue) String() string { return strconv.FormatBool(bool(*cv)) }

func (cv *customBoolFlagValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	*cv = customBoolFlagValue(v)
	return err
}

func (cv *customBoolFlagValue) UnmarshalText(text []byte) error { return cv.Set(string(text)) }
func (cv customBoolFlagValue) MarshalText() ([]byte, error) {
	return []byte(strconv.FormatBool(bool(cv))), nil
}

func (cv *customBoolFlagValue) IsBoolFlag() bool { return true }

func TestPrintDefaults(t *testing.T) {
	fs := NewFlagSet("print defaults test", ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.New[bool]("A", false, "for bootstrapping, allow 'any' type").Ptr()
	fs.New[bool]("Alongflagname", false, "disable bounds checking").Ptr()
	fs.New[bool]("CCC", true, "a boolean defaulting to true").Short('C').Ptr()
	fs.New[string]("D", "", "set relative `path` for local imports").Ptr()
	fs.New[float64]("F", 2.7, "a non-zero `number`").Ptr()
	fs.New[float64]("G", 0, "a float that defaults to zero").Ptr()
	fs.New[int]("N", 27, "a non-zero int").Ptr()
	fs.New[[]int]("Ints", []int{}, "int slice with zero default").Ptr()
	fs.New[net.IP]("IP", nil, "IP address with no default").Ptr()
	fs.New[net.IPMask]("IPMask", nil, "Netmask address with no default").Ptr()
	fs.New[net.IPNet]("IPNet", net.IPNet{}, "IP network with no default").Ptr()
	fs.New[int]("Z", 0, "an int that defaults to zero").Ptr()
	fs.New[time.Duration]("maxT", 0, "set `timeout` for dial").Ptr()
	fs.New[string]("ND1", "foo", "a string with NoOptDefVal").SetNoOptDefVal("bar")
	fs.New[int]("ND2", 1234, "a `num` with NoOptDefVal").SetNoOptDefVal("4321")
	fs.NewP[int]("EEE", "E", 4321, "a `num` with NoOptDefVal").SetNoOptDefVal("1234")
	fs.New[[]string]("StringSlice", []string{}, "string slice with zero default").Ptr()
	_ = (*[]string)(fs.New[[]string]("StringArray", []string([]string{}), "string array with zero default", StringArray()).Ptr())
	_ = (*int)(fs.New[int]("verbose", int(0), "verbosity", Count()).Short('v').Ptr())

	var cv customValue
	fs.NewVar(&cv, "custom", cv, "`custom` Value implementation")

	cv2 := customValue(10)
	fs.NewVarP(&cv2, "customP", "", cv2, "a VarP with `default`")

	// Simulate case where a value has been provided and the help screen is shown
	var cv3 customValue
	fs.NewVar(&cv3, "custom-with-val", cv3, "`custom` value which has been set from command line while help is shown")
	err := fs.Parse([]string{"--custom-with-val", "3"})
	if err != nil {
		t.Error("Parsing flags failed:", err)
	}

	fs.PrintDefaults()
	got := buf.String()
	if got != defaultOutput {
		t.Errorf("\n--- Got:\n%s--- Wanted:\n%s\n", got, defaultOutput)
	}
}

func TestCustomIsBoolFlagDefaultIsZeroValue(t *testing.T) {
	var v customBoolFlagValue
	fs := NewFlagSet("test", ContinueOnError)
	flag := fs.NewVar(&v, "custom-bool", v, "custom bool").Meta()

	if !flag.defaultIsZeroValue() {
		t.Fatal("expected empty default for custom IsBoolFlag value to be treated as zero value")
	}
}

func TestPrintDefaultsCustomIsBoolFlagOmitsDefault(t *testing.T) {
	fs := NewFlagSet("print defaults custom bool", ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)

	var v customBoolFlagValue
	fs.NewVar(&v, "custom-bool", v, "custom bool value implementation")

	fs.PrintDefaults()
	got := buf.String()
	want := "      --custom-bool[=true]   custom bool value implementation\n"
	if got != want {
		t.Errorf("\n--- Got:\n%s--- Wanted:\n%s\n", got, want)
	}
}

func TestVisitAllFlagOrder(t *testing.T) {
	fs := NewFlagSet("TestVisitAllFlagOrder", ContinueOnError)
	fs.SortFlags = false
	// https://github.com/naja-go/gpflag/issues/120
	fs.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
		return NormalizedName(name)
	})

	names := []string{"C", "B", "A", "D"}
	for _, name := range names {
		fs.New[bool](name, false, "").Ptr()
	}

	i := 0
	fs.VisitAll(func(f FlagMeta) {
		if names[i] != f.Name {
			t.Errorf("Incorrect order. Expected %v, got %v", names[i], f.Name)
		}
		i++
	})
}

func TestVisitFlagOrder(t *testing.T) {
	fs := NewFlagSet("TestVisitFlagOrder", ContinueOnError)
	fs.SortFlags = false
	names := []string{"C", "B", "A", "D"}
	for _, name := range names {
		fs.New[bool](name, false, "").Ptr()
		if err := fs.SetText(name, "true"); err != nil {
			t.Error(err)
		}
	}

	i := 0
	fs.Visit(func(f FlagMeta) {
		if names[i] != f.Name {
			t.Errorf("Incorrect order. Expected %v, got %v", names[i], f.Name)
		}
		i++
	})
}
