// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func setUpIP(ip *net.IP) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.NewVar(ip, "address", net.ParseIP("0.0.0.0"), "IP Address")
	return f
}

func TestIP(t *testing.T) {
	testCases := []struct {
		input    string
		success  bool
		expected string
	}{
		{"0.0.0.0", true, "0.0.0.0"},
		{" 0.0.0.0 ", true, "0.0.0.0"},
		{"1.2.3.4", true, "1.2.3.4"},
		{"127.0.0.1", true, "127.0.0.1"},
		{"255.255.255.255", true, "255.255.255.255"},
		{"", true, "0.0.0.0"},
		{"0", false, ""},
		{"localhost", false, ""},
		{"0.0.0", false, ""},
		{"0.0.0.", false, ""},
		{"0.0.0.0.", false, ""},
		{"0.0.0.256", false, ""},
		{"0 . 0 . 0 . 0", false, ""},
	}

	devnull, _ := os.Open(os.DevNull)
	os.Stderr = devnull
	for i := range testCases {
		var addr net.IP
		f := setUpIP(&addr)

		tc := &testCases[i]

		arg := fmt.Sprintf("--address=%s", tc.input)
		err := f.Parse([]string{arg})
		if err != nil && tc.success == true {
			t.Errorf("expected success, got %q", err)
			continue
		} else if err == nil && tc.success == false {
			t.Errorf("expected failure")
			continue
		} else if tc.success {
			ip, err := f.TryGet[net.IP]("address")
			if err != nil {
				t.Errorf("Got error trying to fetch the IP flag: %v", err)
			}
			if ip.String() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, ip.String())
			}
		}
	}
}

func TestIPNilDefault(t *testing.T) {
	var addr net.IP
	f := NewFlagSet("test", ContinueOnError)
	f.NewVar(&addr, "address", net.IP(nil), "IP Address")

	ip, err := f.TryGet[net.IP]("address")
	if err != nil {
		t.Fatalf("expected nil default IP to round-trip without error, got %v", err)
	}
	if ip != nil {
		t.Fatalf("expected nil default IP, got %v", ip)
	}
	if addr != nil {
		t.Fatalf("expected bound IP variable to stay nil, got %v", addr)
	}
}
