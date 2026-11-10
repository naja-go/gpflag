// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag_test

import (
	"fmt"

	"github.com/naja-go/gpflag"
)

func ExampleFlagSet_ShorthandLookup() {
	fs := gpflag.NewFlagSet("example", gpflag.ContinueOnError)
	fs.NewP("verbose", "v", false, "verbose output")
	fmt.Println(fs.ShorthandLookup("v").Name)
	// Output: verbose
}

func ExampleFlagSet_New() {
	fs := gpflag.NewFlagSet("example", gpflag.ContinueOnError)
	fs.New("arg", map[string]string{}, "key=value pairs")
	if err := fs.Parse([]string{"--arg", "a=1,b=2", "--arg", "a=2", "--arg=d=4"}); err != nil {
		panic(err)
	}
	fmt.Println(fs.Get[map[string]string]("arg"))
	// Output: map[a:2 b:2 d:4]
}

func ExampleFlagSet_SetInterspersed() {
	fs := gpflag.NewFlagSet("wrapper", gpflag.ContinueOnError)
	verbose := fs.New("verbose", false, "verbose output")
	fs.SetInterspersed(false)
	if err := fs.Parse([]string{"--verbose", "echo", "--verbose", "hello"}); err != nil {
		panic(err)
	}
	fmt.Println(*verbose)
	fmt.Println(fs.Args())
	// Output:
	// true
	// [echo --verbose hello]
}
