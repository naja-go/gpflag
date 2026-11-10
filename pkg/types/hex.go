// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/hex"
	"strings"
)

// Hex is a byte slice parsed as hexadecimal and displayed in uppercase.
type Hex []byte

// Set decodes hexadecimal input into bytes.
func (b *Hex) Set(s string) error {
	value, err := hex.DecodeString(strings.TrimSpace(s))
	if err == nil {
		*b = value
	}
	return err
}

// String returns uppercase hexadecimal text.
func (b *Hex) String() string { return strings.ToUpper(hex.EncodeToString(*b)) }

// Type returns "bytesHex".
func (b *Hex) Type() string { return "bytesHex" }
