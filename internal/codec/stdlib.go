// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codec

import (
	"encoding/hex"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"
)

// stdlibCodecs keep standard-library types and their usual text syntax. They
// take precedence over underlying kinds: Duration is not an ordinary int64,
// and IP and IPMask are not lists of bytes.
var stdlibCodecs = map[reflect.Type]Codec{
	reflect.TypeFor[time.Duration](): newCodec("duration", `must be a duration like "30s" or "5m"`, time.ParseDuration, time.Duration.String),
	reflect.TypeFor[time.Time]():     newCodec("time", "", parseTime, formatTime),
	reflect.TypeFor[net.IP]():        ipCodec(),
	reflect.TypeFor[net.IPMask]():    newCodec("ipMask", "", ParseIPv4Mask, net.IPMask.String),
	reflect.TypeFor[net.IPNet]():     ipNetCodec(),
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, strings.TrimSpace(s))
}

func formatTime(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.Format(time.RFC3339Nano)
}

func ipCodec() Codec {
	c := newCodec("ip", "", func(s string) (net.IP, error) {
		if v := net.ParseIP(strings.TrimSpace(s)); v != nil {
			return v, nil
		}
		return nil, fmt.Errorf("invalid IP address %q", s)
	}, net.IP.String)
	c.EmptyNoop, c.StripQuotes = true, true
	return c
}

func ipNetCodec() Codec {
	c := newCodec("ipNet", "", func(s string) (net.IPNet, error) {
		_, v, err := net.ParseCIDR(strings.TrimSpace(s))
		if err != nil {
			return net.IPNet{}, err
		}
		return *v, nil
	}, func(v net.IPNet) string { return v.String() })
	c.StripQuotes = true
	return c
}

// ParseIPv4Mask parses a mask written in IP form (e.g. 255.255.255.0) or as
// eight hexadecimal digits (e.g. ffffff00).
//
// Should ideally be apart of stdlib's net package but kept for functionality
// paridy with spf13/pflag
func ParseIPv4Mask(s string) (net.IPMask, error) {
	if ip := net.ParseIP(s).To4(); ip != nil {
		return net.IPv4Mask(ip[0], ip[1], ip[2], ip[3]), nil
	}
	if len(s) == 8 {
		if b, err := hex.DecodeString(s); err == nil {
			return net.IPMask(b), nil
		}
	}
	return nil, fmt.Errorf("invalid IPv4 mask %q", s)
}
