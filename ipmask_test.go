// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"net"
	"reflect"
	"testing"

	"github.com/naja-go/gpflag/internal/codec"
)

func TestParseIPv4Mask(t *testing.T) {
	for _, input := range []string{"255.255.255.0", "ffffff00", "::ffff:255.255.255.0"} {
		if got, err := codec.ParseIPv4Mask(input); err != nil || !reflect.DeepEqual(got, net.IPv4Mask(255, 255, 255, 0)) {
			t.Errorf("ParseIPv4Mask(%q) = %v, want ffffff00", input, got)
		}
	}
	for _, input := range []string{"::", "2001:db8::1"} {
		if got, err := codec.ParseIPv4Mask(input); err == nil || got != nil {
			t.Errorf("ParseIPv4Mask(%q) = %v, want nil", input, got)
		}
	}
}

func TestIPMaskRejectsIPv6(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	mask := f.New("mask", net.IPv4Mask(255, 255, 255, 0), "IPv4 mask")
	if err := f.Set("mask", "2001:db8::1"); err == nil {
		t.Fatal("expected an error for an IPv6 mask")
	}
	if !reflect.DeepEqual(*mask, net.IPv4Mask(255, 255, 255, 0)) {
		t.Fatalf("invalid input changed the mask to %v", *mask)
	}
}
