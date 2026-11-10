// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag_test

import (
	"fmt"

	"github.com/naja-go/gpflag"
	"github.com/naja-go/gpflag/pkg/types"
)

func ExampleNew() {
	fs := gpflag.NewFlagSet("example", gpflag.ContinueOnError)
	port := fs.NewP("port", "p", uint16(8080), "listen port")

	var verbose types.Count
	fs.NewVarP(&verbose, "verbose", "v", types.Count(0), "verbosity")
	if err := fs.Parse([]string{"-vv", "--port=9090"}); err != nil {
		panic(err)
	}

	// Output: 9090 2
	fmt.Println(*port, int(verbose))
}
