// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"bytes"
	goflag "flag"
	"strings"
	"testing"
	"time"
)

func TestGoflags(t *testing.T) {
	stringFlag := goflag.String("stringFlag", "stringFlag", "stringFlag")
	boolFlag := goflag.Bool("boolFlag", false, "boolFlag")
	var testxxxValue string
	goflag.StringVar(&testxxxValue, "test.xxx", "test.xxx", "it is a test flag")

	f := NewFlagSet("test", ContinueOnError)

	f.AddGoFlagSet(goflag.CommandLine)
	args := []string{"--stringFlag=bob", "--boolFlag", "-test.xxx=testvalue"}
	err := f.Parse(args)
	if err != nil {
		t.Fatal("expected no error; get", err)
	}

	getString := *stringFlag
	if getString != "bob" {
		t.Fatalf("expected getString=bob but got getString=%s", getString)
	}

	getBool := *boolFlag
	if getBool != true {
		t.Fatalf("expected getBool=true but got getBool=%v", getBool)
	}
	if !f.Parsed() {
		t.Fatal("f.Parsed() return false after f.Parse() called")
	}

	if testxxxValue != "test.xxx" {
		t.Fatalf("expected testxxxValue to be test.xxx but got %v", testxxxValue)
	}
	err = ParseSkippedFlags(args, goflag.CommandLine)
	if err != nil {
		t.Fatal("expected no error; ParseSkippedFlags", err)
	}
	if testxxxValue != "testvalue" {
		t.Fatalf("expected testxxxValue to be testvalue but got %v", testxxxValue)
	}

	// in fact it is useless. because `go test` called flag.Parse()
	if !goflag.CommandLine.Parsed() {
		t.Fatal("goflag.CommandLine.Parsed() return false after f.Parse() called")
	}
}

func TestToGoflags(t *testing.T) {
	pfs := FlagSet{}
	gfs := goflag.FlagSet{}
	pfs.New[string]("StringFlag", "String value", "String flag usage").Ptr()
	pfs.New[int]("IntFlag", 1, "Int flag usage").Ptr()
	pfs.New[uint]("UintFlag", 2, "Uint flag usage").Ptr()
	pfs.New[int64]("Int64Flag", 3, "Int64 flag usage").Ptr()
	pfs.New[uint64]("Uint64Flag", 4, "Uint64 flag usage").Ptr()
	pfs.New[int8]("Int8Flag", 5, "Int8 flag usage").Ptr()
	pfs.New[float64]("Float64Flag", 6.0, "Float64 flag usage").Ptr()
	pfs.New[time.Duration]("DurationFlag", time.Second, "Duration flag usage").Ptr()
	pfs.New[bool]("BoolFlag", true, "Bool flag usage").Ptr()
	pfs.New[string]("deprecated", "Deprecated value", "Deprecated flag usage").Ptr()
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
		pf := pfs.lookupCore(name)
		if pf == nil {
			t.Errorf("%s: not found in pflag flag set", name)
			continue
		}
		if err := pfs.SetText(pf.Name, value); err != nil {
			t.Errorf("error setting %s = %s: %v", name, value, err)
		}
	}

	// Check that all flags were added and share the same value.
	pfs.VisitAll(func(pf FlagMeta) {
		gf := gfs.Lookup(pf.Name)
		if gf == nil {
			t.Errorf("%s: not found in Go flag set", pf.Name)
			return
		}
		if gf.Value.String() != pfs.formal[NormalizedName(pf.Name)].text() {
			t.Errorf("%s: expected value %v from Go flag set, got %v",
				pf.Name, pfs.formal[NormalizedName(pf.Name)].text(), gf.Value)
			return
		}
	})

	// Check for unexpected additional flags.
	gfs.VisitAll(func(gf *goflag.Flag) {
		pf := pfs.lookupCore(gf.Name)
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

	var help bytes.Buffer
	gfs.SetOutput(&help)
	gfs.PrintDefaults()
	if strings.Contains(help.String(), "panic") {
		t.Fatalf("copied flags produced invalid help: %s", help.String())
	}
	for _, want := range []string{"-StringFlag string", "-IntFlag int", "-UintFlag uint", "-DurationFlag duration", "-Float64Flag float"} {
		if !strings.Contains(help.String(), want) {
			t.Errorf("copied help missing %q: %s", want, help.String())
		}
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
			value := pfs.New[string]("name", "default", "a string flag").Ptr()
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
			if got := pfs.lookupCore("name").DefValue; got != "default" {
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
			if err := pfs.SetText("name", "from-pflag"); err != nil {
				t.Fatal(err)
			}
			if got := gfs.Lookup("name").Value.String(); got != "from-pflag" {
				t.Errorf("pflag update not shared with copy: got %q", got)
			}
		})
	}
}
