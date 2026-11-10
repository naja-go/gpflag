// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type textString string

func (s *textString) UnmarshalText(text []byte) error {
	*s = textString(text)
	if strings.Contains(string(text), "invalid") {
		return fmt.Errorf("invalid text")
	}
	return nil
}
func (s textString) MarshalText() ([]byte, error) { return []byte(s), nil }

func TestText(t *testing.T) {
	for _, input := range []string{
		"2003-01-02T15:04:05Z", "2024-11-22T03:01:02Z", "2006-01-02T15:04:05+07:00",
	} {
		t.Run(input, func(t *testing.T) {
			f := NewFlagSet("text", ContinueOnError)
			var value time.Time
			f.NewVar(&value, "time", time.Time{}, "timestamp")
			if err := f.Parse([]string{"--time=" + input}); err != nil {
				t.Fatal(err)
			}
			got, err := f.TryGet[time.Time]("time")
			want, _ := time.Parse(time.RFC3339, input)
			if err != nil || !got.Equal(want) || !value.Equal(want) {
				t.Fatalf("got %v, %v; want %v", got, err, want)
			}
			if _, err := f.TryGet[textString]("time"); err == nil {
				t.Fatal("Get accepted an unrelated text type")
			}
			if err := f.Set("time", "2003-01-02 15:05:01"); err == nil || !value.Equal(want) {
				t.Fatal("invalid time was accepted or changed storage")
			}
		})
	}
}

func TestTextMarshalerPrecedesUnderlyingKind(t *testing.T) {
	f := NewFlagSet("text", ContinueOnError)
	value := textString("default")
	f.NewVar(&value, "text", value, "")
	if err := f.Set("text", "valid"); err != nil || value != "valid" {
		t.Fatalf("got %q, %v", value, err)
	}
	if err := f.Set("text", "invalid"); err == nil || value != "valid" {
		t.Fatal("failed UnmarshalText modified the bound value")
	}
}
