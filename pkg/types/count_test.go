// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types_test

import (
	"strconv"
	"testing"

	"github.com/naja-go/gpflag/pkg/types"
)

func TestCount(t *testing.T) {
	count := types.Count(0)
	for _, text := range []string{"+1", "+1", "0x2a", "+1"} {
		if err := count.Set(text); err != nil {
			t.Fatal(err)
		}
	}
	if count != 43 || count.String() != "43" {
		t.Fatalf("got %v, want 43", count)
	}
	if err := count.Set("invalid"); err == nil || count != 43 {
		t.Fatal("invalid count was accepted or changed storage")
	}
	maxInt := int(^uint(0) >> 1)
	if err := count.Set(strconv.Itoa(maxInt)); err != nil {
		t.Fatal(err)
	}
	if err := count.Set("+1"); err == nil || count != types.Count(maxInt) {
		t.Fatal("increment overflow was accepted or changed storage")
	}
}
