// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types_test

import (
	"testing"

	"github.com/naja-go/gpflag/pkg/types"
)

func TestBoolFunc(t *testing.T) {
	var got string
	value := types.BoolFunc(func(s string) error { got = s; return nil })
	if err := value.Set("true"); err != nil || got != "true" {
		t.Fatalf("callback = %q, %v", got, err)
	}
	if !value.IsBoolFlag() || value.String() != "" || value.Type() != "boolfunc" {
		t.Fatal("incorrect callback metadata")
	}
}
