// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"strings"
	"time"
)

// Time accepts any of the supplied Go time layouts. Use time.Time directly when
// RFC3339 is sufficient. String always uses RFC3339Nano, preserving precision.
type Time struct {
	time.Time
	Formats []string
}

// Set sets the parsed time using configured formats
func (t *Time) Set(s string) error {
	for _, layout := range t.Formats {
		value, err := time.Parse(layout, strings.TrimSpace(s))
		if err == nil {
			t.Time = value
			return nil
		}
	}
	return fmt.Errorf("invalid time %q; expected one of %q", s, t.Formats)
}

// String returns the formatted RFC3339Nano string
func (t *Time) String() string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

// Type returns the type string
func (t *Time) Type() string { return "time" }
