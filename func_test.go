// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"strings"
	"testing"
)

func cmpLists(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFunc(t *testing.T) {
	var values []string
	fn := func(s string) error {
		values = append(values, s)
		return nil
	}

	fset := NewFlagSet("test", ContinueOnError)
	fset.New("fnflag", fn, "Callback function")

	err := fset.Parse([]string{"--fnflag=aa", "--fnflag", "bb"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	expected := []string{"aa", "bb"}
	if !cmpLists(expected, values) {
		t.Fatalf("expected %v, got %v", expected, values)
	}
}

func TestFuncP(t *testing.T) {
	var values []string
	fn := func(s string) error {
		values = append(values, s)
		return nil
	}

	fset := NewFlagSet("test", ContinueOnError)
	fset.NewP("fnflag", "f", fn, "Callback function")

	err := fset.Parse([]string{"--fnflag=a", "--fnflag", "b", "-fc", "-f=d", "-f", "e"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	expected := []string{"a", "b", "c", "d", "e"}
	if !cmpLists(expected, values) {
		t.Fatalf("expected %v, got %v", expected, values)
	}
}

func TestFuncUsage(t *testing.T) {
	t.Run("regular func flag", func(t *testing.T) {
		// regular func flag:
		// expect to see '--flag1 value' followed by the usageMessage, and no mention of a default value
		fset := NewFlagSet("unittest", ContinueOnError)
		fset.New("flag1", func(string) error { return nil }, "usage message")
		usage := fset.FlagUsagesWrapped(80)

		usage = strings.TrimSpace(usage)
		expected := "--flag1 value   usage message"
		if usage != expected {
			t.Fatalf("unexpected generated usage message\n  expected: %s\n       got: %s", expected, usage)
		}
	})

	t.Run("func flag with placeholder name", func(t *testing.T) {
		// func flag, with a placeholder name:
		// if usageMesage contains a placeholder, expect that name; still expect no mention of a default value
		fset := NewFlagSet("unittest", ContinueOnError)
		fset.New("flag2", func(string) error { return nil }, "usage message with `name` placeholder")
		usage := fset.FlagUsagesWrapped(80)

		usage = strings.TrimSpace(usage)
		expected := "--flag2 name   usage message with name placeholder"
		if usage != expected {
			t.Fatalf("unexpected generated usage message\n  expected: %s\n       got: %s", expected, usage)
		}
	})
}
