// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"strings"
	"testing"

	"github.com/naja-go/gpflag/pkg/types"
)

func TestBoolFunc(t *testing.T) {
	var count int
	fn := func(_ string) error {
		count++
		return nil
	}

	fset := NewFlagSet("test", ContinueOnError)
	fset.New("func", types.BoolFunc(fn), "Callback function")

	err := fset.Parse([]string{"--func", "--func=1", "--func=false"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if count != 3 {
		t.Fatalf("expected 3 calls to the callback, got %d calls", count)
	}
}

func TestBoolFuncP(t *testing.T) {
	var count int
	fn := func(_ string) error {
		count++
		return nil
	}

	fset := NewFlagSet("test", ContinueOnError)
	fset.NewP("bfunc", "b", types.BoolFunc(fn), "Callback function")

	err := fset.Parse([]string{"--bfunc", "--bfunc=0", "--bfunc=false", "-b", "-b=0"})
	if err != nil {
		t.Fatal("expected no error; got", err)
	}

	if count != 5 {
		t.Fatalf("expected 5 calls to the callback, got %d calls", count)
	}
}

func TestBoolFuncUsage(t *testing.T) {
	t.Run("regular func flag", func(t *testing.T) {
		fn := func(string) error { return nil }

		// regular boolfunc flag:
		// expect to see '--flag1' followed by the usageMessage, and no mention of a default value
		fset := NewFlagSet("unittest", ContinueOnError)
		fset.New("flag1", types.BoolFunc(fn), "usage message")
		usage := fset.FlagUsagesWrapped(80)

		usage = strings.TrimSpace(usage)
		expected := "--flag1   usage message"
		if usage != expected {
			t.Fatalf("unexpected generated usage message\n  expected: %s\n       got: %s", expected, usage)
		}
	})

	t.Run("func flag with placeholder name", func(t *testing.T) {
		fn := func(string) error { return nil }
		// func flag, with a placeholder name:
		// if usageMesage contains a placeholder, expect '--flag2 {placeholder}'; still expect no mention of a default value
		fset := NewFlagSet("unittest", ContinueOnError)
		fset.New("flag2", types.BoolFunc(fn), "usage message with `name` placeholder")
		usage := fset.FlagUsagesWrapped(80)

		usage = strings.TrimSpace(usage)
		expected := "--flag2 name   usage message with name placeholder"
		if usage != expected {
			t.Fatalf("unexpected generated usage message\n  expected: %s\n       got: %s", expected, usage)
		}
	})
}
