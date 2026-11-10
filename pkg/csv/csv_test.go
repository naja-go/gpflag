// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package csv

import (
	"reflect"
	"testing"
)

func TestCSVRoundTrip(t *testing.T) {
	for _, fields := range [][]string{
		{},
		{""},
		{"", ""},
		{"a", "b"},
		{"has,comma", `has"quote`},
		{"line\nbreak"},
	} {
		text := Format(fields)
		got, err := Read(text)
		if err != nil || !reflect.DeepEqual(got, fields) {
			t.Errorf("Read(Format(%q)) = %q, %v via %q", fields, got, err, text)
		}
	}
}

func TestReadCSVRejectsMultipleRecords(t *testing.T) {
	if _, err := Read("a\nb"); err == nil {
		t.Fatal("two records were accepted")
	}
}
