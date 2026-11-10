// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/base64"
	"strings"
)

// Base64 is a byte slice parsed and displayed using standard padded base64.
type Base64 []byte

// Set sets the base64 value given a string input
func (b *Base64) Set(s string) error {
	value, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err == nil {
		*b = value
	}
	return err
}

// String returns the base64 encoded string
func (b *Base64) String() string { return base64.StdEncoding.EncodeToString(*b) }

// Type returns the string type for the base64 type
func (b *Base64) Type() string { return "base64" }
