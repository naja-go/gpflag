// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import "errors"

// BoolFunc is a callback flag that accepts boolean no-argument syntax. For a
// callback requiring an argument, use func(string) error directly with gpflag.New.
// The callback is invoked on each occurrence, and its error stops parsing.
type BoolFunc func(string) error

// Set calls the callback with s. A nil callback is an error.
func (f *BoolFunc) Set(s string) error {
	if *f == nil {
		return errors.New("gpflag: nil callback")
	}
	return (*f)(s)
}

// String returns an empty string; callbacks have no default text.
func (f *BoolFunc) String() string { return "" }

// Type returns "boolfunc".
func (f *BoolFunc) Type() string { return "boolfunc" }

// IsBoolFlag enables boolean no-argument syntax.
func (f *BoolFunc) IsBoolFlag() bool { return true }
