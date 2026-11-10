// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag_test

import (
	"errors"
	"math/big"
	"net"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	flag "github.com/naja-go/gpflag"
)

type portNumber uint16
type enabledFlag bool
type messageFlag string
type portList []portNumber
type labelKey string
type portMap map[labelKey]portNumber

func TestNamedNativeValues(t *testing.T) {
	fs := flag.NewFlagSet("named", flag.ContinueOnError)
	p := newFlag(fs, "port", portNumber(8080), "port")
	b := fs.NewP("enabled", "e", enabledFlag(false), "enabled")
	s := newFlag(fs, "message", messageFlag("0"), "message")
	fs.New("ports", portList{1}, "ports")
	fs.New("map", portMap{"default": 1}, "map")
	if err := fs.Parse([]string{"--port=9090", "-e", "--ports=2,3", "--ports=4", "--map=a=2", "--map=b=3"}); err != nil {
		t.Fatal(err)
	}
	if got, err := p.TryGet[portNumber](); err != nil || got != 9090 {
		t.Fatalf("port = %v, %v", got, err)
	}
	if _, err := p.TryGet[uint16](); err == nil {
		t.Fatal("Get discarded a defined type")
	}
	if got := *b; !got {
		t.Fatalf("enabled = %v", got)
	}
	if got, err := s.TryGet[messageFlag](); err != nil || got != "0" {
		t.Fatalf("message = %v, %v", got, err)
	}
	if got, err := fs.TryGet[portList]("ports"); err != nil || !reflect.DeepEqual(got, portList{2, 3, 4}) {
		t.Fatalf("ports = %v, %v", got, err)
	}
	if got, err := fs.TryGet[portMap]("map"); err != nil || !reflect.DeepEqual(got, portMap{"a": 2, "b": 3}) {
		t.Fatalf("map = %v, %v", got, err)
	}
	if err := fs.Set("port", "65536"); err == nil {
		t.Fatal("accepted a named uint16 overflow")
	}
	s.NoOptDefVal = "false"
	usage := fs.FlagUsages()
	if !strings.Contains(usage, "--enabled[=true|false]") || !strings.Contains(usage, `(default "0")`) || !strings.Contains(usage, `[="false"]`) {
		t.Fatalf("named primitives lost native help behavior:\n%s", usage)
	}
}

func TestCollectionErrorsAreAtomic(t *testing.T) {
	fs := flag.NewFlagSet("atomic", flag.ContinueOnError)
	s := newFlag(fs, "slice", []uint8{7}, "")
	m := newFlag(fs, "map", map[string]uint8{"default": 7}, "")
	for _, text := range []string{"1,256", `"unterminated`, "1\n2"} {
		if err := fs.Set("slice", text); err == nil {
			t.Fatalf("accepted invalid slice %q", text)
		}
		got, _ := s.TryGet[[]uint8]()
		if !reflect.DeepEqual(got, []uint8{7}) || s.Changed {
			t.Fatalf("failed slice parse changed default/state: %v, %v", got, s.Changed)
		}
	}
	for _, text := range []string{"a=1,b=256", "missing-equals", "a=1\nb=2"} {
		if err := fs.Set("map", text); err == nil {
			t.Fatalf("accepted invalid map %q", text)
		}
		got, _ := m.TryGet[map[string]uint8]()
		if !reflect.DeepEqual(got, map[string]uint8{"default": 7}) || m.Changed {
			t.Fatalf("failed map parse changed default/state: %v, %v", got, m.Changed)
		}
	}
	if err := fs.Parse([]string{"--slice=1", "--map=a=1"}); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("slice", "2,256"); err == nil {
		t.Fatal("accepted invalid repeated slice")
	}
	if err := fs.Set("map", "b=2,c=256"); err == nil {
		t.Fatal("accepted invalid repeated map")
	}
	if err := fs.Parse([]string{"--slice=2", "--map=b=2"}); err != nil {
		t.Fatal(err)
	}
	gotSlice, _ := s.TryGet[[]uint8]()
	gotMap, _ := m.TryGet[map[string]uint8]()
	if !reflect.DeepEqual(gotSlice, []uint8{1, 2}) || !reflect.DeepEqual(gotMap, map[string]uint8{"a": 1, "b": 2}) {
		t.Fatalf("first-success/repeated semantics lost: %v, %v", gotSlice, gotMap)
	}
	slice := s.Value.(flag.SliceValue)
	for _, err := range []error{slice.Append("256"), slice.Replace([]string{"3", "256"})} {
		if err == nil {
			t.Fatal("SliceValue accepted overflow")
		}
	}
	gotSlice, _ = s.TryGet[[]uint8]()
	if !reflect.DeepEqual(gotSlice, []uint8{1, 2}) {
		t.Fatalf("failed SliceValue operation changed storage: %v", gotSlice)
	}
}

func TestCollectionDefaultsAndNil(t *testing.T) {
	defaults := []string{"default"}
	mapping := map[string]string{"default": "value"}
	fs := flag.NewFlagSet("defaults", flag.ContinueOnError)
	a := newFlag(fs, "a", defaults, "")
	b := newFlag(fs, "b", defaults, "")
	m := newFlag(fs, "map", mapping, "")
	aValue, _ := a.TryGet[[]string]()
	aValue[0] = "changed"
	if got, _ := b.TryGet[[]string](); got[0] != "default" || defaults[0] != "default" {
		t.Fatal("slice defaults share working storage")
	}
	mValue, _ := m.TryGet[map[string]string]()
	mValue["default"] = "changed"
	if mapping["default"] != "value" {
		t.Fatal("map defaults share working storage")
	}
	nilSlice := newFlag(fs, "nil-slice", []string(nil), "")
	emptySlice := newFlag(fs, "empty-slice", []string{}, "")
	nilMap := newFlag(fs, "nil-map", map[string]int(nil), "")
	emptyMap := newFlag(fs, "empty-map", map[string]int{}, "")
	ns, _ := nilSlice.TryGet[[]string]()
	es, _ := emptySlice.TryGet[[]string]()
	nm, _ := nilMap.TryGet[map[string]int]()
	em, _ := emptyMap.TryGet[map[string]int]()
	if ns != nil || es == nil || nm != nil || em == nil {
		t.Fatal("nil/empty defaults were conflated")
	}
	if err := fs.Parse([]string{"--nil-slice=", "--nil-map="}); err != nil {
		t.Fatal(err)
	}
	ns, _ = nilSlice.TryGet[[]string]()
	nm, _ = nilMap.TryGet[map[string]int]()
	if ns == nil || len(ns) != 0 || nm == nil || len(nm) != 0 {
		t.Fatal("explicit empty collections should be empty, not nil")
	}
	if m.DefValue != "[default=value]" {
		t.Fatal("default text changed with working storage")
	}
}

func TestStandardLibraryValues(t *testing.T) {
	fs := flag.NewFlagSet("standard", flag.ContinueOnError)
	d := fs.New("duration", time.Second, "")
	fs.New("durations", []time.Duration(nil), "")
	fs.New("address", netip.Addr{}, "")
	fs.New("prefixes", []netip.Prefix{}, "")
	fs.New("big", (*big.Int)(nil), "")
	if err := fs.Parse([]string{"--duration=2m", "--durations=1s,2m", "--address=2001:db8::1", "--prefixes=192.0.2.0/24", "--big=123456789012345678901234567890"}); err != nil {
		t.Fatal(err)
	}
	if got := *d; got != 2*time.Minute {
		t.Fatalf("duration = %v", got)
	}
	if got, err := fs.TryGet[[]time.Duration]("durations"); err != nil || !reflect.DeepEqual(got, []time.Duration{time.Second, 2 * time.Minute}) {
		t.Fatalf("durations = %v, %v", got, err)
	}
	if got, err := fs.TryGet[netip.Addr]("address"); err != nil || got != netip.MustParseAddr("2001:db8::1") {
		t.Fatalf("address = %v, %v", got, err)
	}
	if got, err := fs.TryGet[[]netip.Prefix]("prefixes"); err != nil || !reflect.DeepEqual(got, []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}) {
		t.Fatalf("prefixes = %v, %v", got, err)
	}
	if got, err := fs.TryGet[*big.Int]("big"); err != nil || got.String() != "123456789012345678901234567890" {
		t.Fatalf("big = %v, %v", got, err)
	}
	_, network, _ := net.ParseCIDR("192.0.2.0/24")
	networkFlag := newFlag(fs, "network", *network, "")
	stored, _ := networkFlag.TryGet[net.IPNet]()
	stored.IP[0] = 1
	stored.Mask[0] = 0
	if network.IP[0] != 192 || network.Mask[0] != 255 {
		t.Fatal("network default collections were not copied")
	}
}

func TestNativeErrorsRetainGoCauses(t *testing.T) {
	fs := flag.NewFlagSet("causes", flag.ContinueOnError)
	fs.New("port", portNumber(1), "")
	err := fs.Parse([]string{"--port=65536"})
	var number *strconv.NumError
	if !errors.Is(err, strconv.ErrRange) || !errors.As(err, &number) {
		t.Fatalf("native parse cause was lost: %v", err)
	}
	var invalid *flag.InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("structured flag error was lost: %v", err)
	}
}
