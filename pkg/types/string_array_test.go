// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types_test

import (
	"reflect"
	"testing"

	"github.com/naja-go/gpflag/pkg/types"
)

func TestStringArray(t *testing.T) {
	value := types.StringArray{"default"}
	if err := value.SetFirst("first,whole", true); err != nil {
		t.Fatal(err)
	}
	if err := value.Set("second"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, types.StringArray{"first,whole", "second"}) {
		t.Fatalf("got %v", value)
	}
	if value.String() != `["first,whole",second]` || value.Type() != "stringArray" {
		t.Fatalf("unexpected formatting: %s", value.String())
	}
	if err := value.Replace([]string{""}); err != nil || value.String() != `[""]` {
		t.Fatalf("empty element formatting: %s, %v", value.String(), err)
	}
	sliceCopy := value.GetSlice()
	sliceCopy[0] = "changed"
	if value[0] != "" {
		t.Fatal("GetSlice exposes mutable internal storage")
	}
}
