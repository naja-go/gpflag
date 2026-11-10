// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"bytes"
	goflag "flag"
	"testing"
	"time"
)

func TestGoflags(t *testing.T) {
	gfs := goflag.NewFlagSet("go", goflag.ContinueOnError)
	gfs.String("stringFlag", "stringFlag", "stringFlag")
	gfs.Bool("boolFlag", false, "boolFlag")
	var testxxxValue string
	gfs.StringVar(&testxxxValue, "test.xxx", "test.xxx", "it is a test flag")

	f := NewFlagSet("test", ContinueOnError)

	f.AddGoFlagSet(gfs)
	args := []string{"--stringFlag=bob", "--boolFlag", "-test.xxx=testvalue"}
	err := f.Parse(args)
	if err != nil {
		t.Fatal("expected no error; get", err)
	}

	getString, err := f.TryGet[string]("stringFlag")
	if err != nil {
		t.Fatal("expected no error; get", err)
	}
	if getString != "bob" {
		t.Fatalf("expected getString=bob but got getString=%s", getString)
	}

	getBool, err := f.TryGet[bool]("boolFlag")
	if err != nil {
		t.Fatal("expected no error; get", err)
	}
	if getBool != true {
		t.Fatalf("expected getBool=true but got getBool=%v", getBool)
	}
	if !f.Parsed() {
		t.Fatal("f.Parsed() return false after f.Parse() called")
	}

	if testxxxValue != "test.xxx" {
		t.Fatalf("expected testxxxValue to be test.xxx but got %v", testxxxValue)
	}
	err = ParseSkippedFlags(args, gfs)
	if err != nil {
		t.Fatal("expected no error; ParseSkippedFlags", err)
	}
	if testxxxValue != "testvalue" {
		t.Fatalf("expected testxxxValue to be testvalue but got %v", testxxxValue)
	}

	if !gfs.Parsed() {
		t.Fatal("gfs.Parsed() return false after f.Parse() called")
	}
}

func TestToGoflags(t *testing.T) {
	pfs := &FlagSet{}
	gfs := goflag.FlagSet{}
	pfs.New("StringFlag", "String value", "String flag usage")
	pfs.New("IntFlag", 1, "Int flag usage")
	pfs.New("UintFlag", uint(2), "Uint flag usage")
	pfs.New("Int64Flag", int64(3), "Int64 flag usage")
	pfs.New("Uint64Flag", uint64(4), "Uint64 flag usage")
	pfs.New("Int8Flag", int8(5), "Int8 flag usage")
	pfs.New("Float64Flag", float64(6.0), "Float64 flag usage")
	pfs.New("DurationFlag", time.Second, "Duration flag usage")
	pfs.New("BoolFlag", true, "Bool flag usage")
	pfs.New("deprecated", "Deprecated value", "Deprecated flag usage")
	_ = pfs.MarkDeprecated("deprecated", "obsolete")

	pfs.CopyToGoFlagSet(&gfs)

	// Modify via pfs. Should be visible via gfs because both share the
	// same values.
	for name, value := range map[string]string{
		"StringFlag":  "Modified String value",
		"IntFlag":     "11",
		"UintFlag":    "12",
		"Int64Flag":   "13",
		"Uint64Flag":  "14",
		"Int8Flag":    "15",
		"Float64Flag": "16.0",
		"BoolFlag":    "false",
	} {
		pf := pfs.Lookup(name)
		if pf == nil {
			t.Errorf("%s: not found in pflag flag set", name)
			continue
		}
		if err := pf.Value.Set(value); err != nil {
			t.Errorf("error setting %s = %s: %v", name, value, err)
		}
	}

	// Check that all flags were added and share the same value.
	pfs.VisitAll(func(pf *Flag) {
		gf := gfs.Lookup(pf.Name)
		if gf == nil {
			t.Errorf("%s: not found in Go flag set", pf.Name)
			return
		}
		if gf.Value.String() != pf.Value.String() {
			t.Errorf("%s: expected value %v from Go flag set, got %v",
				pf.Name, pf.Value, gf.Value)
			return
		}
	})

	// Check for unexpected additional flags.
	gfs.VisitAll(func(gf *goflag.Flag) {
		pf := pfs.Lookup(gf.Name)
		if pf == nil {
			t.Errorf("%s: not found in pflag flag set", gf.Name)
			return
		}
	})

	deprecated := gfs.Lookup("deprecated")
	if deprecated == nil {
		t.Error("deprecated: not found in Go flag set")
	} else {
		expectedUsage := "Deprecated flag usage (DEPRECATED: obsolete)"
		if deprecated.Usage != expectedUsage {
			t.Errorf("deprecation remark not added, expected usage %q, got %q", expectedUsage, deprecated.Usage)
		}
	}
}

func TestCopyToGoFlagSetNativeHelp(t *testing.T) {
	pfs := NewFlagSet("source", ContinueOnError)
	pfs.New("number", 0, "an integer")
	pfs.New("text", "hello", "a string")
	pfs.New("enabled", false, "a boolean")
	pfs.New("timeout", time.Duration(0), "a duration")
	copied := goflag.NewFlagSet("copied", goflag.ContinueOnError)
	pfs.CopyToGoFlagSet(copied)

	standard := goflag.NewFlagSet("standard", goflag.ContinueOnError)
	standard.Int("number", 0, "an integer")
	standard.String("text", "hello", "a string")
	standard.Bool("enabled", false, "a boolean")
	standard.Duration("timeout", 0, "a duration")
	var got, want bytes.Buffer
	copied.SetOutput(&got)
	standard.SetOutput(&want)
	copied.PrintDefaults()
	standard.PrintDefaults()
	if got.String() != want.String() {
		t.Fatalf("native Go flag help changed:\ngot:\n%swant:\n%s", got.String(), want.String())
	}
}

func TestCopyToGoFlagSetPreservesCurrentStringValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "default", want: "default"},
		{name: "parsed", args: []string{"--name=parsed"}, want: "parsed"},
		{name: "empty", args: []string{"--name="}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pfs := NewFlagSet("source", ContinueOnError)
			value := pfs.New("name", "default", "a string flag")
			if err := pfs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			gfs := goflag.NewFlagSet("destination", goflag.ContinueOnError)
			pfs.CopyToGoFlagSet(gfs)
			if *value != tc.want {
				t.Fatalf("copy changed source value: got %q, want %q", *value, tc.want)
			}
			if got := gfs.Lookup("name").Value.String(); got != tc.want {
				t.Errorf("copied value = %q, want %q", got, tc.want)
			}
			if got := pfs.Lookup("name").DefValue; got != "default" {
				t.Errorf("source default = %q, want %q", got, "default")
			}
			if got := gfs.Lookup("name").DefValue; got != tc.want {
				t.Errorf("copied default = %q, want current value %q", got, tc.want)
			}
			if err := gfs.Set("name", "from-go"); err != nil {
				t.Fatal(err)
			}
			if *value != "from-go" {
				t.Errorf("Go flag update not shared with source: got %q", *value)
			}
			if err := pfs.Set("name", "from-pflag"); err != nil {
				t.Fatal(err)
			}
			if got := gfs.Lookup("name").Value.String(); got != "from-pflag" {
				t.Errorf("pflag update not shared with copy: got %q", got)
			}
		})
	}
}
