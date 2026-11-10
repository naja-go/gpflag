// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types_test

import (
	"testing"

	"github.com/naja-go/gpflag/pkg/types"
)

func TestBase64(t *testing.T) {
	var value types.Base64
	for _, test := range []struct{ text, want string }{
		{"aGVsbG8=", "hello"},
		{"AP8=", "\x00\xff"},
		{"", ""},
	} {
		if err := value.Set(test.text); err != nil {
			t.Fatal(err)
		}
		if string(value) != test.want || value.String() != test.text {
			t.Fatalf("got %q (%s), want %q (%s)", value, value.String(), test.want, test.text)
		}
	}
	value = types.Base64("unchanged")
	if err := value.Set("aGVsbG8=!!!"); err == nil || string(value) != "unchanged" {
		t.Fatal("partial decode was accepted or changed storage")
	}
}
