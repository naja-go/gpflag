// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types_test

import (
	"reflect"
	"testing"

	"github.com/naja-go/gpflag/pkg/types"
)

func TestHex(t *testing.T) {
	value := types.Hex("old")
	if err := value.Set(" 00ff "); err != nil || !reflect.DeepEqual(value, types.Hex{0, 255}) {
		t.Fatalf("hex = %v, %v", value, err)
	}
	if value.String() != "00FF" || value.Type() != "bytesHex" {
		t.Fatalf("unexpected formatting: %s", value.String())
	}
	for _, invalid := range []string{"0", "abc", "gg"} {
		if err := value.Set(invalid); err == nil || value.String() != "00FF" {
			t.Fatal("invalid hex was accepted or changed storage")
		}
	}
}
