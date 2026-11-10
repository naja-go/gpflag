// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/naja-go/gpflag/pkg/types"
)

var normalizeFlagNameInvocations = 0

func boolString(s string) string {
	if s == "0" {
		return "false"
	}
	return "true"
}

// newTestFlagSet returns an empty flag set that discards its output, returns
// parse errors rather than exiting, and reports usage through the given func.
func newTestFlagSet(usage func()) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.SetOutput(io.Discard)
	f.Usage = usage
	return f
}

func TestEverything(t *testing.T) {
	fs := NewFlagSet("everything", ContinueOnError)
	fs.New("test_bool", false, "bool value")
	fs.New("test_int", 0, "int value")
	fs.New("test_int64", int64(0), "int64 value")
	fs.New("test_uint", uint(0), "uint value")
	fs.New("test_uint64", uint64(0), "uint64 value")
	fs.New("test_string", "0", "string value")
	fs.New("test_float64", float64(0), "float64 value")
	fs.New("test_duration", time.Duration(0), "time.Duration value")
	fs.New("test_optional_int", 0, "optional int value")

	m := make(map[string]*Flag)
	desired := "0"
	visitor := func(f *Flag) {
		if len(f.Name) > 5 && f.Name[0:5] == "test_" {
			m[f.Name] = f
			ok := false
			switch {
			case f.Value.String() == desired:
				ok = true
			case f.Name == "test_bool" && f.Value.String() == boolString(desired):
				ok = true
			case f.Name == "test_duration" && f.Value.String() == desired+"s":
				ok = true
			}
			if !ok {
				t.Error("Visit: bad value", f.Value.String(), "for", f.Name)
			}
		}
	}
	fs.VisitAll(visitor)
	if len(m) != 9 {
		t.Error("VisitAll misses some flags")
		for k, v := range m {
			t.Log(k, *v)
		}
	}
	m = make(map[string]*Flag)
	fs.Visit(visitor)
	if len(m) != 0 {
		t.Errorf("Visit sees unset flags")
		for k, v := range m {
			t.Log(k, *v)
		}
	}
	// Now set all flags
	if err := fs.Set("test_bool", "true"); err != nil {
		t.Error(err)
	}
	if err := fs.Set("test_int", "1"); err != nil {
		t.Error(err)
	}
	if err := fs.Set("test_int64", "1"); err != nil {
		t.Error(err)
	}
	if err := fs.Set("test_uint", "1"); err != nil {
		t.Error(err)
	}
	if err := fs.Set("test_uint64", "1"); err != nil {
		t.Error(err)
	}
	if err := fs.Set("test_string", "1"); err != nil {
		t.Error(err)
	}
	if err := fs.Set("test_float64", "1"); err != nil {
		t.Error(err)
	}
	if err := fs.Set("test_duration", "1s"); err != nil {
		t.Error(err)
	}
	if err := fs.Set("test_optional_int", "1"); err != nil {
		t.Error(err)
	}
	desired = "1"
	fs.Visit(visitor)
	if len(m) != 9 {
		t.Error("Visit fails after set")
		for k, v := range m {
			t.Log(k, *v)
		}
	}
	// Now test they're visited in sort order.
	var flagNames []string
	fs.Visit(func(f *Flag) { flagNames = append(flagNames, f.Name) })
	if !slices.IsSorted(flagNames) {
		t.Errorf("flag names not sorted: %v", flagNames)
	}
}

func TestNoArgument(t *testing.T) {
	if NewFlagSet("test", ContinueOnError).Parse([]string{}) != nil {
		t.Error("parse failed for empty argument list")
	}
}

func TestUsage(t *testing.T) {
	called := false
	fs := newTestFlagSet(func() { called = true })
	err := fs.Parse([]string{"--x"})
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

	oldSet.New("flag1", "flag1", "flag1")
	oldSet.New("flag2", "flag2", "flag2")

	newSet.New("flag2", "flag2", "flag2")
	newSet.New("flag3", "flag3", "flag3")

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
	f.NewP("stringa", "a", "", "string value")
	if err := f.SetAnnotation("stringa", "key", nil); err != nil {
		t.Errorf("Unexpected error setting new nil annotation: %v", err)
	}
	if annotation := f.Lookup("stringa").Annotations["key"]; annotation != nil {
		t.Errorf("Unexpected annotation: %v", annotation)
	}
	f.NewP("stringb", "b", "", "string2 value")
	if err := f.SetAnnotation("stringb", "key", []string{"value1"}); err != nil {
		t.Errorf("Unexpected error setting new annotation: %v", err)
	}
	if annotation := f.Lookup("stringb").Annotations["key"]; !reflect.DeepEqual(annotation, []string{"value1"}) {
		t.Errorf("Unexpected annotation: %v", annotation)
	}

	if err := f.SetAnnotation("stringb", "key", []string{"value2"}); err != nil {
		t.Errorf("Unexpected error updating annotation: %v", err)
	}
	if annotation := f.Lookup("stringb").Annotations["key"]; !reflect.DeepEqual(annotation, []string{"value2"}) {
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
	boolFlag := f.New("bool", false, "bool value")
	bool2Flag := f.New("bool2", false, "bool2 value")
	bool3Flag := f.New("bool3", false, "bool3 value")
	intFlag := f.New("int", 0, "int value")
	int8Flag := f.New("int8", int8(0), "int value")
	int16Flag := f.New("int16", int16(0), "int value")
	int32Flag := f.New("int32", int32(0), "int value")
	int64Flag := f.New("int64", int64(0), "int64 value")
	uintFlag := f.New("uint", uint(0), "uint value")
	uint8Flag := f.New("uint8", uint8(0), "uint value")
	uint16Flag := f.New("uint16", uint16(0), "uint value")
	uint32Flag := f.New("uint32", uint32(0), "uint value")
	uint64Flag := f.New("uint64", uint64(0), "uint64 value")
	stringFlag := f.New("string", "0", "string value")
	float32Flag := f.New("float32", float32(0), "float32 value")
	float64Flag := f.New("float64", float64(0), "float64 value")
	ipFlag := f.New("ip", net.ParseIP("127.0.0.1"), "ip value")
	maskFlag := f.New("mask", net.IPv4Mask(0, 0, 0, 0), "mask value")
	durationFlag := f.New("duration", 5*time.Second, "time.Duration value")
	optionalIntNoValueFlag := f.New("optional-int-no-value", 0, "int value")
	f.Lookup("optional-int-no-value").NoOptDefVal = "9"
	optionalIntWithValueFlag := f.New("optional-int-with-value", 0, "int value")
	f.Lookup("optional-int-no-value").NoOptDefVal = "9"
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
	boolValue := *boolFlag
	if boolValue != true {
		t.Error("bool flag should be true, is ", boolValue)
	}
	if v, err := f.TryGet[bool]("bool"); err != nil || v != boolValue {
		t.Error("TryGet[bool] does not work.")
	}
	bool2Value := *bool2Flag
	if bool2Value != true {
		t.Error("bool2 flag should be true, is ", bool2Value)
	}
	bool3Value := *bool3Flag
	if bool3Value != false {
		t.Error("bool3 flag should be false, is ", bool3Value)
	}
	intValue := *intFlag
	if intValue != 22 {
		t.Error("int flag should be 22, is ", intValue)
	}
	if v, err := f.TryGet[int]("int"); err != nil || v != intValue {
		t.Error("TryGet[int] does not work.")
	}
	int8Value := *int8Flag
	if int8Value != -8 {
		t.Error("int8 flag should be -8, is ", int8Value)
	}
	int16Value := *int16Flag
	if int16Value != -16 {
		t.Error("int16 flag should be -16, is ", int16Value)
	}
	if v, err := f.TryGet[int8]("int8"); err != nil || v != int8Value {
		t.Error("TryGet[int8] does not work.")
	}
	if v, err := f.TryGet[int16]("int16"); err != nil || v != int16Value {
		t.Error("TryGet[int16] does not work.")
	}
	int32Value := *int32Flag
	if int32Value != -32 {
		t.Error("int32 flag should be -32, is ", int32Value)
	}
	if v, err := f.TryGet[int32]("int32"); err != nil || v != int32Value {
		t.Error("TryGet[int32] does not work.")
	}
	int64Value := *int64Flag
	if int64Value != 0x23 {
		t.Error("int64 flag should be 0x23, is ", int64Value)
	}
	if v, err := f.TryGet[int64]("int64"); err != nil || v != int64Value {
		t.Error("TryGet[int64] does not work.")
	}
	uintValue := *uintFlag
	if uintValue != 24 {
		t.Error("uint flag should be 24, is ", uintValue)
	}
	if v, err := f.TryGet[uint]("uint"); err != nil || v != uintValue {
		t.Error("TryGet[uint] does not work.")
	}
	uint8Value := *uint8Flag
	if uint8Value != 8 {
		t.Error("uint8 flag should be 8, is ", uint8Value)
	}
	if v, err := f.TryGet[uint8]("uint8"); err != nil || v != uint8Value {
		t.Error("TryGet[uint8] does not work.")
	}
	uint16Value := *uint16Flag
	if uint16Value != 16 {
		t.Error("uint16 flag should be 16, is ", uint16Value)
	}
	if v, err := f.TryGet[uint16]("uint16"); err != nil || v != uint16Value {
		t.Error("TryGet[uint16] does not work.")
	}
	uint32Value := *uint32Flag
	if uint32Value != 32 {
		t.Error("uint32 flag should be 32, is ", uint32Value)
	}
	if v, err := f.TryGet[uint32]("uint32"); err != nil || v != uint32Value {
		t.Error("TryGet[uint32] does not work.")
	}
	uint64Value := *uint64Flag
	if uint64Value != 25 {
		t.Error("uint64 flag should be 25, is ", uint64Value)
	}
	if v, err := f.TryGet[uint64]("uint64"); err != nil || v != uint64Value {
		t.Error("TryGet[uint64] does not work.")
	}
	stringValue := *stringFlag
	if stringValue != "hello" {
		t.Error("string flag should be `hello`, is ", stringValue)
	}
	if v, err := f.TryGet[string]("string"); err != nil || v != stringValue {
		t.Error("TryGet[string] does not work.")
	}
	float32Value := *float32Flag
	if float32Value != -172e12 {
		t.Error("float32 flag should be -172e12, is ", float32Value)
	}
	if v, err := f.TryGet[float32]("float32"); err != nil || v != float32Value {
		t.Errorf("TryGet[float32] returned %v but float32Flag was %v", v, float32Value)
	}
	float64Value := *float64Flag
	if float64Value != 2718e28 {
		t.Error("float64 flag should be 2718e28, is ", float64Value)
	}
	if v, err := f.TryGet[float64]("float64"); err != nil || v != float64Value {
		t.Errorf("TryGet[float64] returned %v but float64Flag was %v", v, float64Value)
	}
	ipValue := *ipFlag
	if !ipValue.Equal(net.ParseIP("10.11.12.13")) {
		t.Error("ip flag should be 10.11.12.13, is ", ipValue)
	}
	if v, err := f.TryGet[net.IP]("ip"); err != nil || !v.Equal(ipValue) {
		t.Errorf("TryGet[net.IP] returned %v but ipFlag was %v", v, ipValue)
	}
	maskValue := *maskFlag
	if maskValue.String() != net.IPv4Mask(255, 255, 255, 0).String() {
		t.Error("mask flag should be 255.255.255.0, is ", maskValue.String())
	}
	if v, err := f.TryGet[net.IPMask]("mask"); err != nil || v.String() != maskValue.String() {
		t.Errorf("TryGet[net.IPMask] returned %v maskFlag was %v error was %v", v, maskValue, err)
	}
	durationValue := *durationFlag
	if durationValue != 2*time.Minute {
		t.Error("duration flag should be 2m, is ", durationValue)
	}
	if v, err := f.TryGet[time.Duration]("duration"); err != nil || v != durationValue {
		t.Error("TryGet[time.Duration] does not work.")
	}
	if _, err := f.TryGet[int]("duration"); err == nil {
		t.Error("GetInt parsed a time.Duration?!?!")
	}
	optionalIntNoValue := *optionalIntNoValueFlag
	if optionalIntNoValue != 9 {
		t.Error("optional int flag should be the default value, is ", optionalIntNoValue)
	}
	optionalIntWithValue := *optionalIntWithValueFlag
	if optionalIntWithValue != 42 {
		t.Error("optional int flag should be 42, is ", optionalIntWithValue)
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
	f.NewP("boola", "a", false, "bool value")
	f.NewP("boolb", "b", false, "bool2 value")
	f.NewP("boolc", "c", false, "bool3 value")
	f.NewP("boold", "d", false, "bool4 value")
	f.NewP("stringa", "s", "0", "string value")
	f.NewP("stringz", "z", "0", "string value")
	f.NewP("stringx", "x", "0", "string value")
	f.NewP("stringy", "y", "0", "string value")
	f.Lookup("stringx").NoOptDefVal = "1"
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
	store := func(flag *Flag, value string) error {
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
	f.NewP("boola", "a", false, "bool value")
	f.NewP("boolb", "b", false, "bool2 value")
	f.NewP("boolc", "c", false, "bool3 value")
	f.NewP("boold", "d", false, "bool4 value")
	f.NewP("boole", "e", false, "bool4 value")
	f.NewP("stringa", "s", "0", "string value")
	f.NewP("stringz", "z", "0", "string value")
	f.NewP("stringx", "x", "0", "string value")
	f.NewP("stringy", "y", "0", "string value")
	f.NewP("stringo", "o", "0", "string value")
	f.Lookup("stringx").NoOptDefVal = "1"
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
	store := func(flag *Flag, value string) error {
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
	f.NewP("boola", "a", false, "bool value")
	f.NewP("boolb", "b", false, "bool2 value")
	f.NewP("boolc", "c", false, "bool3 value")
	f.NewP("boold", "d", false, "bool4 value")
	f.NewP("boole", "e", false, "bool4 value")
	f.NewP("stringa", "s", "0", "string value")
	f.NewP("stringz", "z", "0", "string value")
	f.NewP("stringx", "x", "0", "string value")
	f.NewP("stringy", "y", "0", "string value")
	f.NewP("stringo", "o", "0", "string value")
	f.Lookup("stringx").NoOptDefVal = "1"
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
	store := func(flag *Flag, value string) error {
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
	boolaFlag := f.NewP("boola", "a", false, "bool value")
	boolbFlag := f.NewP("boolb", "b", false, "bool2 value")
	boolcFlag := f.NewP("boolc", "c", false, "bool3 value")
	booldFlag := f.NewP("boold", "d", false, "bool4 value")
	stringaFlag := f.NewP("stringa", "s", "0", "string value")
	stringzFlag := f.NewP("stringz", "z", "0", "string value")
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
	f.SetOutput(io.Discard)
	if err := f.Parse(args); err != nil {
		t.Error("expected no error, got ", err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	boolaValue := *boolaFlag
	if boolaValue != true {
		t.Error("boola flag should be true, is ", boolaValue)
	}
	boolbValue := *boolbFlag
	if boolbValue != true {
		t.Error("boolb flag should be true, is ", boolbValue)
	}
	boolcValue := *boolcFlag
	if boolcValue != true {
		t.Error("boolc flag should be true, is ", boolcValue)
	}
	booldValue := *booldFlag
	if booldValue != true {
		t.Error("boold flag should be true, is ", booldValue)
	}
	stringaValue := *stringaFlag
	if stringaValue != "hello" {
		t.Error("stringa flag should be `hello`, is ", stringaValue)
	}
	stringzValue := *stringzFlag
	if stringzValue != "something" {
		t.Error("stringz flag should be `something`, is ", stringzValue)
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
	f.NewP("boola", "a", false, "bool value")
	f.NewP("boolb", "b", false, "bool2 value")
	args := []string{
		"-ab",
	}
	f.SetOutput(io.Discard)
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
		{"bool", func(f *FlagSet) { f.New("bool", false, "") }, "x",
			`invalid argument "x" for "--bool" flag: must be true or false`},
		{"int", func(f *FlagSet) { f.New("int", 0, "") }, "x",
			`invalid argument "x" for "--int" flag: must be an integer`},
		{"uint", func(f *FlagSet) { f.New("uint", uint(0), "") }, "-1",
			`invalid argument "-1" for "--uint" flag: must be a non-negative integer`},
		{"float64", func(f *FlagSet) { f.New("float64", float64(0), "") }, "x",
			`invalid argument "x" for "--float64" flag: must be a number`},
		{"duration", func(f *FlagSet) { f.New("duration", time.Duration(0), "") }, "soon",
			`invalid argument "soon" for "--duration" flag: must be a duration like "30s" or "5m"`},
	}
	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			f := NewFlagSet("test", ContinueOnError)
			f.SetOutput(io.Discard)
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
	testParseAll(newTestFlagSet(func() { t.Error("bad parse") }), t)
}

func TestIgnoreUnknownFlags(t *testing.T) {
	testParseWithUnknownFlags(newTestFlagSet(func() { t.Error("bad parse") }), t, func(f *FlagSet) { f.ParseErrorsAllowlist.UnknownFlagsHandling = IgnoreUnknownFlag })
}

func TestIgnoreUnknownFlagsAndPassToArgs(t *testing.T) {
	testParseWithUnknownFlagsAndPassToArgs(newTestFlagSet(func() { t.Error("bad parse") }), t)
}

func TestParse(t *testing.T) {
	testParse(newTestFlagSet(func() { t.Error("bad parse") }), t)
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
	f.New("changed", false, "changed bool")
	f.New("settrue", true, "true to true")
	f.New("setfalse", false, "false to false")
	f.New("unchanged", false, "unchanged bool")

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
	withDashFlag := f.New("with-dash-flag", false, "bool value")
	// Set this after some flags have been added and before others.
	f.SetNormalizeFunc(wordSepNormalizeFunc)
	withUnderFlag := f.New("with_under_flag", false, "bool value")
	withBothFlag := f.New("with-both_flag", false, "bool value")
	if err := f.Parse(args); err != nil {
		t.Fatal(err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	withDashValue := *withDashFlag
	if withDashValue != true {
		t.Error("withDashFlag flag should be true, is ", withDashValue)
	}
	withUnderValue := *withUnderFlag
	if withUnderValue != true {
		t.Error("withUnderFlag flag should be true, is ", withUnderValue)
	}
	withBothValue := *withBothFlag
	if withBothValue != true {
		t.Error("withBothFlag flag should be true, is ", withBothValue)
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

	validFlag := f.New("valid-flag", false, "bool value")
	f.SetNormalizeFunc(aliasAndWordSepFlagNames)
	someOtherFlag := f.New("some-other-flag", false, "bool value")

	args := []string{"--old_valid_flag", "--some-other_flag"}
	if err := f.Parse(args); err != nil {
		t.Fatal(err)
	}

	validValue := *validFlag
	if validValue != true {
		t.Errorf("validFlag is %v even though we set the alias --old_valid_falg", validValue)
	}
	someOtherValue := *someOtherFlag
	if someOtherValue != true {
		t.Error("someOtherFlag should be true, is ", someOtherValue)
	}
}

// Every flag we add, the name (displayed also in usage) should normalized
func TestNormalizationFuncShouldChangeFlagName(t *testing.T) {
	// Test normalization after addition
	f := NewFlagSet("normalized", ContinueOnError)
	f.New("valid_flag", false, "bool value")
	if f.Lookup("valid_flag").Name != "valid_flag" {
		t.Error("The new flag should have the name 'valid_flag' instead of ", f.Lookup("valid_flag").Name)
	}

	f.SetNormalizeFunc(wordSepNormalizeFunc)
	if f.Lookup("valid_flag").Name != "valid.flag" {
		t.Error("The new flag should have the name 'valid.flag' instead of ", f.Lookup("valid_flag").Name)
	}

	// Test normalization before addition
	f = NewFlagSet("normalized", ContinueOnError)
	f.SetNormalizeFunc(wordSepNormalizeFunc)
	f.New("valid_flag", false, "bool value")
	if f.Lookup("valid_flag").Name != "valid.flag" {
		t.Error("The new flag should have the name 'valid.flag' instead of ", f.Lookup("valid_flag").Name)
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
	f.New(testName, false, "bool value")
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

	if !reflect.DeepEqual(f.formal, g.formal) || !reflect.DeepEqual(f.orderedFormal, g.orderedFormal) {
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
	f.New(testName, false, "bool value")
	if err := f.Set(testName, "true"); err != nil {
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

func (f *flagVar) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (f *flagVar) Type() string {
	return "flagVar"
}

func TestUserDefined(t *testing.T) {
	flags := NewFlagSet("test", ContinueOnError)
	var v flagVar
	flags.NewP("v", "v", &v, "usage")
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
	flags := NewFlagSet("test", ContinueOnError)
	var buf bytes.Buffer
	flags.SetOutput(&buf)

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

// Test that -help invokes the usage message and returns ErrHelp.
func TestHelp(t *testing.T) {
	helpCalled := false
	fs := NewFlagSet("help test", ContinueOnError)
	fs.Usage = func() { helpCalled = true }
	var flag bool
	fs.NewVar(&flag, "flag", bool(false), "regular flag")
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
	if err != ErrHelp { //nolint:errorlint // Parse must return the exact ErrHelp sentinel.
		t.Fatal("expected ErrHelp; got ", err)
	}
	if !helpCalled {
		t.Fatal("help was not called")
	}
	// If we define a help flag, that should override.
	var help bool
	fs.NewVar(&help, "help", bool(false), "help flag")
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
	f.New("true", true, "always true")
	f.New("false", false, "always false")
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
	boolFlag := f.NewP("bool", "l", false, "bool value")
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
	f.SetOutput(io.Discard)
	if err := f.Parse(args); err != nil {
		t.Fatal("expected no error; got ", err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	boolValue := *boolFlag
	if boolValue {
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
	f.New("badflag", true, "always true")
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
	flg := f.Lookup("badflag")
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
	f.NewP(name, "n", true, "always true")
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
	f.New("badflag", true, "always true")
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
	f.NewP(name, "n", true, "always true")
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
	f.New("bad-double_flag", true, "always true")
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
	// not the canonical name. See https://github.com/spf13/pflag/issues/279.
	f := NewFlagSet("bob", ContinueOnError)
	f.New("src", "", "src directory or file")
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
	f.New("with_under_flag", false, "bool value")

	if normalizeFlagNameInvocations != 1 {
		t.Fatal("Expected normalizeFlagNameInvocations to be 1; got ", normalizeFlagNameInvocations)
	}
}

func TestHiddenFlagInUsage(t *testing.T) {
	f := NewFlagSet("bob", ContinueOnError)
	f.New("secretFlag", true, "shhh")
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
	f.New("secretFlag", true, "shhh")
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
      --customP custom               a VarP with default (default 10)
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

func (cv *customValue) Type() string { return "custom" }

type customBoolFlagValue bool

func (cv *customBoolFlagValue) String() string { return strconv.FormatBool(bool(*cv)) }

func (cv *customBoolFlagValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	*cv = customBoolFlagValue(v)
	return err
}

func (cv *customBoolFlagValue) Type() string { return "custom-bool" }

func (cv *customBoolFlagValue) IsBoolFlag() bool { return true }

func TestPrintDefaults(t *testing.T) {
	fs := NewFlagSet("print defaults test", ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.New("A", false, "for bootstrapping, allow 'any' type")
	fs.New("Alongflagname", false, "disable bounds checking")
	fs.NewP("CCC", "C", true, "a boolean defaulting to true")
	fs.New("D", "", "set relative `path` for local imports")
	fs.New("F", 2.7, "a non-zero `number`")
	fs.New("G", float64(0), "a float that defaults to zero")
	fs.New("N", 27, "a non-zero int")
	fs.New("Ints", []int{}, "int slice with zero default")
	fs.New[net.IP]("IP", nil, "IP address with no default")
	fs.New[net.IPMask]("IPMask", nil, "Netmask address with no default")
	fs.New("IPNet", net.IPNet{}, "IP network with no default")
	fs.New("Z", 0, "an int that defaults to zero")
	fs.New("maxT", time.Duration(0), "set `timeout` for dial")
	fs.New("ND1", "foo", "a string with NoOptDefVal")
	fs.Lookup("ND1").NoOptDefVal = "bar"
	fs.New("ND2", 1234, "a `num` with NoOptDefVal")
	fs.Lookup("ND2").NoOptDefVal = "4321"
	fs.NewP("EEE", "E", 4321, "a `num` with NoOptDefVal")
	fs.ShorthandLookup("E").NoOptDefVal = "1234"
	fs.New("StringSlice", []string{}, "string slice with zero default")
	fs.New("StringArray", types.StringArray{}, "string array with zero default")
	fs.NewP("verbose", "v", types.Count(0), "verbosity")

	var cv customValue
	fs.New("custom", &cv, "custom Value implementation")

	cv2 := customValue(10)
	fs.NewP("customP", "", &cv2, "a VarP with default")

	// Simulate case where a value has been provided and the help screen is shown
	var cv3 customValue
	fs.New("custom-with-val", &cv3, "custom value which has been set from command line while help is shown")
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
	flag := &Flag{
		Name:     "custom-bool",
		Usage:    "custom bool",
		Value:    &v,
		DefValue: "",
	}

	if !flag.defaultIsZeroValue() {
		t.Fatal("expected empty default for custom IsBoolFlag value to be treated as zero value")
	}
}

func TestPrintDefaultsCustomIsBoolFlagOmitsDefault(t *testing.T) {
	fs := NewFlagSet("print defaults custom bool", ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)

	var v customBoolFlagValue
	fs.New("custom-bool", &v, "custom bool value implementation")

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
	// https://github.com/spf13/pflag/issues/120
	fs.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
		return NormalizedName(name)
	})

	names := []string{"C", "B", "A", "D"}
	for _, name := range names {
		fs.New(name, false, "")
	}

	i := 0
	fs.VisitAll(func(f *Flag) {
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
		fs.New(name, false, "")
		if err := fs.Set(name, "true"); err != nil {
			t.Error(err)
		}
	}

	i := 0
	fs.Visit(func(f *Flag) {
		if names[i] != f.Name {
			t.Errorf("Incorrect order. Expected %v, got %v", names[i], f.Name)
		}
		i++
	})
}
