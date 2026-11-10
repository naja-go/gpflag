// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"errors"
	goflag "flag"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/naja-go/gpflag/pkg/types"
)

func TestCallbackStandardBehavior(t *testing.T) {
	cause := errors.New("callback error")
	for _, test := range []struct {
		name    string
		boolean bool
		args    []string
		want    []string
		fail    bool
	}{
		{"values", false, []string{"--fn=xx", "--fn", "yy", "--fn=zz"}, []string{"xx", "yy", "zz"}, false},
		{"boolean", true, []string{"--fn", "--fn=false", "--fn=0"}, []string{"true", "false", "0"}, false},
		{"error", false, []string{"--fn=before", "--fn=err", "--fn=after"}, []string{"before", "err"}, true},
		{"boolean-error", true, []string{"--fn", "--fn=err", "--fn"}, []string{"true", "err"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var standard, generic []string
			callback := func(values *[]string) func(string) error {
				return func(s string) error {
					*values = append(*values, s)
					if s == "err" {
						return cause
					}
					return nil
				}
			}
			std := goflag.NewFlagSet("standard", goflag.ContinueOnError)
			std.SetOutput(io.Discard)
			fs := NewFlagSet("generic", ContinueOnError)
			if test.boolean {
				std.BoolFunc("fn", "callback", callback(&standard))
				fs.New("fn", types.BoolFunc(callback(&generic)), "callback")
			} else {
				std.Func("fn", "callback", callback(&standard))
				fs.New("fn", callback(&generic), "callback")
			}
			stdErr, err := std.Parse(test.args), fs.Parse(test.args)
			if (stdErr != nil) != test.fail || (err != nil) != test.fail {
				t.Fatalf("unexpected errors: standard=%v, generic=%v", stdErr, err)
			}
			if test.fail && (!errors.Is(err, cause) || !strings.HasSuffix(stdErr.Error(), cause.Error())) {
				t.Fatalf("lost callback error: standard=%v, generic=%v", stdErr, err)
			}
			if !reflect.DeepEqual(standard, test.want) || !reflect.DeepEqual(generic, test.want) {
				t.Fatalf("standard=%v, generic=%v; want %v", standard, generic, test.want)
			}
		})
	}
}

func TestNamedCallback(t *testing.T) {
	type action func(string) error
	var got string
	fs := NewFlagSet("callback", ContinueOnError)
	fs.New("fn", action(func(s string) error { got = s; return nil }), "")
	if err := fs.Parse([]string{"--fn=value"}); err != nil || got != "value" {
		t.Fatalf("named callback = %q, %v", got, err)
	}
}
