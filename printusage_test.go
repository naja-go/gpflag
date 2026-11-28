// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"bytes"
	goflag "flag"
	"io"
	"testing"
)

const expectedOutput = `      --long-form[=true|false]    Some description
      --long-form2[=true|false]   Some description
                                    with multiline
  -s, --long-name[=true|false]    Some description
  -t, --long-name2[=true|false]   Some description with
                                    multiline
`

func setUpPFlagSet(buf io.Writer) *FlagSet {
	f := NewFlagSet("test", ExitOnError)
	f.New("long-form", false, "Some description")
	f.New("long-form2", false, "Some description\n  with multiline")
	f.NewP("long-name", "s", false, "Some description")
	f.NewP("long-name2", "t", false, "Some description with\n  multiline")
	f.SetOutput(buf)
	return f
}

func TestPrintUsage(t *testing.T) {
	buf := bytes.Buffer{}
	f := setUpPFlagSet(&buf)
	f.PrintDefaults()
	res := buf.String()
	if res != expectedOutput {
		t.Errorf("Expected \n%s \nActual \n%s", expectedOutput, res)
	}
}

func setUpPFlagSet2(buf io.Writer) *FlagSet {
	f := NewFlagSet("test", ExitOnError)
	f.New("long-form", false, "Some description")
	f.New("long-form2", false, "Some description\n  with multiline")
	f.NewP("long-name", "s", false, "Some description")
	f.NewP("long-name2", "t", false, "Some description with\n  multiline")
	f.NewP("some-very-long-arg", "l", "test", "Some very long description having break the limit")
	f.NewP("other-very-long-arg", "o", "long-default-value", "Some very long description having break the limit")
	f.New("some-very-long-arg2", "very long default value", "Some very long description\nwith line break\nmultiple")
	f.SetOutput(buf)
	return f
}

const expectedOutput2 = `      --long-form[=true|false]       Some description
      --long-form2[=true|false]      Some description
                                       with multiline
  -s, --long-name[=true|false]       Some description
  -t, --long-name2[=true|false]      Some description with
                                       multiline
  -o, --other-very-long-arg string   Some very long description having
                                     break the limit (default
                                     "long-default-value")
  -l, --some-very-long-arg string    Some very long description having
                                     break the limit (default "test")
      --some-very-long-arg2 string   Some very long description
                                     with line break
                                     multiple (default "very long default
                                     value")
`

func TestPrintUsage_2(t *testing.T) {
	buf := bytes.Buffer{}
	f := setUpPFlagSet2(&buf)
	res := f.FlagUsagesWrapped(80)
	if res != expectedOutput2 {
		t.Errorf("Expected \n%s \nActual \n%s", expectedOutput2, res)
	}
}

// A word wider than the description column must sit on its own line.
// Wrapping has to resume for the words after it (#501).
func TestFlagUsagesWrappedContinuesAfterUnbreakableWord(t *testing.T) {
	f := NewFlagSet("example", ContinueOnError)
	f.New("mount", "", "a mount specification e.g. 'type=bind,source=/opt,destination=/hostopt'. The rest of this description is never wrapped.")
	got := f.FlagUsagesWrapped(60)
	want := `      --mount string   a mount specification e.g.
                       'type=bind,source=/opt,destination=/hostopt'.
                       The rest of this description is
                       never wrapped.
`
	if got != want {
		t.Errorf("FlagUsagesWrapped(60)\nwant:\n%q\ngot:\n%q", want, got)
	}
}

// The expected text is the output of spf13/pflag v1.0.10 for the same flags.
func TestLegacyUsageFormat(t *testing.T) {
	g := goflag.NewFlagSet("go", goflag.ContinueOnError)
	g.Duration("go-duration", 0, "a go duration")

	f := NewFlagSet("test", ContinueOnError)
	f.LegacyUsageFormat = true
	f.AddGoFlagSet(g)
	f.New("bool", false, "a bool")
	f.New[[]bool]("bools", nil, "a bool slice")
	f.New[[]string]("strings", nil, "a string slice")
	var custom customBoolFlagValue
	f.New("custom-bool", &custom, "a custom bool")

	want := `      --bool                             a bool
      --bools bools                      a bool slice (default [])
      --custom-bool custom-bool[=true]   a custom bool
      --go-duration duration             a go duration (default 0s)
      --strings strings                  a string slice
`
	if got := f.FlagUsages(); got != want {
		t.Errorf("want:\n%s\ngot:\n%s", want, got)
	}
}
