// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package csv implements CSV helper functions for parsing, reading, and
// formatting string CSV data.
package csv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Read parses exactly one CSV record. The empty string is an empty list.
func Read(text string) ([]string, error) {
	if text == "" {
		return []string{}, nil
	}
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	fields, err := r.Read()
	if err != nil {
		return nil, err
	}
	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("must be a single CSV record; quote fields containing newlines")
	}
	return fields, nil
}

// Format writes fields as one CSV record. A single empty field is quoted so
// it stays distinguishable from an empty list.
func Format(fields []string) string {
	if len(fields) == 1 && fields[0] == "" {
		return `""`
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write(fields)
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n")
}
