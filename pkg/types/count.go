// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"math"
	"strconv"
)

// Count is an integer flag that increments when used without an argument.
type Count int

// Set increments the count for "+1" or sets it to the parsed integer.
func (c *Count) Set(s string) error {
	if s == "+1" {
		if *c == math.MaxInt {
			return fmt.Errorf("count overflows int")
		}
		*c++
	} else {
		value, err := strconv.ParseInt(s, 0, strconv.IntSize)
		if err != nil {
			return err
		}
		*c = Count(value)
	}

	return nil
}

// String returns the decimal count.
func (c *Count) String() string { return strconv.Itoa(int(*c)) }

// Int returns the int type count.
func (c *Count) Int() int { return int(*c) }

// Type returns "count".
func (c *Count) Type() string { return "count" }

// NoOptDefValue returns the increment marker "+1".
func (c *Count) NoOptDefValue() string { return "+1" }
