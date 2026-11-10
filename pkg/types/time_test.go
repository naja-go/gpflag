// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types_test

import (
	"testing"
	"time"

	"github.com/naja-go/gpflag/pkg/types"
)

func TestTime(t *testing.T) {
	value := types.Time{Formats: []string{time.RFC3339Nano, time.DateOnly}}
	if value.String() != "" || value.Type() != "time" {
		t.Fatal("zero time should be empty")
	}
	if err := value.Set(" 2026-01-02 "); err != nil || value.String() != "2026-01-02T00:00:00Z" {
		t.Fatalf("time = %s, %v", value.String(), err)
	}
	previous := value.Time
	if err := value.Set("bad"); err == nil || !value.Time.Equal(previous) {
		t.Fatal("invalid time was accepted or changed storage")
	}
}
