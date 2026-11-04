package gpflag

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func setUpIPNet(ip *net.IPNet) (*FlagSet, *Flag[net.IPNet]) {
	f := NewFlagSet("test", ContinueOnError)
	_, def, _ := net.ParseCIDR("0.0.0.0/0")
	flag := f.NewVar(ip, "address", *def, "IP Address")
	return f, flag
}

func TestIPNet(t *testing.T) {
	testCases := []struct {
		input    string
		success  bool
		expected string
	}{
		{"0.0.0.0/0", true, "0.0.0.0/0"},
		{" 0.0.0.0/0 ", true, "0.0.0.0/0"},
		{"1.2.3.4/8", true, "1.0.0.0/8"},
		{"127.0.0.1/16", true, "127.0.0.0/16"},
		{"255.255.255.255/19", true, "255.255.224.0/19"},
		{"255.255.255.255/32", true, "255.255.255.255/32"},
		{"", false, ""},
		{"/0", false, ""},
		{"0", false, ""},
		{"0/0", false, ""},
		{"localhost/0", false, ""},
		{"0.0.0/4", false, ""},
		{"0.0.0./8", false, ""},
		{"0.0.0.0./12", false, ""},
		{"0.0.0.256/16", false, ""},
		{"0.0.0.0 /20", false, ""},
		{"0.0.0.0/ 24", false, ""},
		{"0 . 0 . 0 . 0 / 28", false, ""},
		{"0.0.0.0/33", false, ""},
	}

	devnull, _ := os.Open(os.DevNull)
	os.Stderr = devnull
	for i := range testCases {
		var addr net.IPNet
		f, flag := setUpIPNet(&addr)

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
			flag, err = f.Lookup[net.IPNet]("address")
			if err != nil {
				t.Fatal("Lookup[net.IPNet]:", err)
			}
			ip := flag.Get()
			if ip.String() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, ip.String())
			}
		}
	}
}
