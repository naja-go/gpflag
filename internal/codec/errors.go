// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codec

import "errors"

// hintedError replaces a low-level parse message (such as strconv's) with a
// user-facing hint, while keeping the original error available to errors.Is/As.
type hintedError struct {
	hint  string
	cause error
}

func (e *hintedError) Error() string { return e.hint }
func (e *hintedError) Unwrap() error { return e.cause }

func withHint(err error, hint string) error {
	if hint == "" {
		return err
	}
	if _, ok := errors.AsType[*hintedError](err); ok {
		return err
	}
	return &hintedError{hint: hint, cause: err}
}
