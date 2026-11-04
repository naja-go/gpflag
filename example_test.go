// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag_test

import (
	"fmt"

	"github.com/naja-go/gpflag"
)

func ExampleShorthandLookup() {
	name := "verbose"

	gpflag.New(name, false, "verbose output").Short('v')

	// len(short) must be == 1
	flag := gpflag.ShorthandLookup("v")

	fmt.Println(flag.Name)
}

func ExampleFlagSet_NewP() {
	name := "verbose"

	fs := gpflag.NewFlagSet("Example", gpflag.ContinueOnError)
	fs.NewP(name, "v", false, "verbose output")

	// len(short) must be == 1
	flag := fs.ShorthandLookup("v")

	fmt.Println(flag.Name)
}

func ExampleFlagSet_Lookup() {
	fs := gpflag.NewFlagSet("Example", gpflag.ContinueOnError)
	fs.New[[]bool]("bs", nil, "booleans")
	if err := fs.Parse([]string{"--bs=true,false"}); err != nil {
		panic(err)
	}
	bs, err := fs.Lookup[[]bool]("bs")
	if err != nil {
		panic(err)
	}
	fmt.Println(bs.Get())
	// Output: [true false]
}

func ExampleFlagSet_New_map() {
	args := []string{
		"--arg", "a=1,b=2",
		"--arg", "a=2",
		"--arg=d=4",
	}

	fs := gpflag.NewFlagSet("Example", gpflag.ContinueOnError)
	arg := fs.New("arg", map[string]string{}, "string-to-string arg accepting key=value pairs")

	if err := fs.Parse(args); err != nil {
		panic(err)
	}

	fmt.Println(arg.Get())
	// Output: map[a:2 b:2 d:4]
}

func ExampleFlagSet_New_bool() {
	flags := gpflag.NewFlagSet("wrapper", gpflag.ContinueOnError)
	verbose := flags.New("verbose", false, "verbose output")
	flags.SetInterspersed(false)
	if err := flags.Parse([]string{"--verbose", "echo", "--verbose", "hello"}); err != nil {
		panic(err)
	}
	fmt.Println(verbose.Get())
	fmt.Println(flags.Args())
	// Output:
	// true
	// [echo --verbose hello]
}
