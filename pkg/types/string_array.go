// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"bytes"
	"encoding/csv"
	"strings"
)

// StringArray treats each occurrence as one string, without splitting on commas.
// In a gpflag flag, the first occurrence replaces the default and later ones
// append. Use []string instead for CSV splitting.
type StringArray []string

// Type returns "stringArray".
func (s *StringArray) Type() string { return "stringArray" }

// String returns the values as bracketed CSV.
func (s *StringArray) String() string {
	if len(*s) == 1 && (*s)[0] == "" {
		return `[""]`
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write([]string(*s))
	w.Flush()
	return "[" + strings.TrimSuffix(b.String(), "\n") + "]"
}

// Set appends one string. FlagSets call SetFirst instead, to replace defaults.
func (s *StringArray) Set(text string) error { return s.Append(text) }

// SetFirst is the gpflag hook for occurrence-aware custom parsing.
func (s *StringArray) SetFirst(text string, first bool) error {
	if first {
		*s = StringArray{text}
	} else {
		*s = append(*s, text)
	}
	return nil
}

// Append adds one string without splitting it.
func (s *StringArray) Append(text string) error {
	*s = append(*s, text)
	return nil
}

// Replace overwrites the values with a copy of text.
func (s *StringArray) Replace(text []string) error {
	*s = append(StringArray{}, text...)
	return nil
}

// GetSlice returns a copy of the values.
func (s *StringArray) GetSlice() []string { return append([]string{}, *s...) }
