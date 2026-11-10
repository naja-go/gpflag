// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/naja-go/gpflag/internal/codec"
	"github.com/naja-go/gpflag/pkg/csv"
)

// mapValue stores a map[K]V where K has kind string and V has a scalar codec.
//
// A mapValue collection accepts CSV records of key=value pairs. The first
// successful occurrence of the flag replaces the whole default map; later
// occurrences merge into it, overwriting keys that repeat.
//
// For example, given
//
//	limits := fs.New("limit", map[string]int{"cpu": 1}, "resource limits")
//
// the command line
//
//	--limit=mem=4,disk=10 --limit=mem=8
//
// yields map[string]int{"mem": 8, "disk": 10}: the first occurrence clears
// the default map, so "cpu" is gone, and the second occurrence merges over
// the first. An occurrence that fails to parse changes nothing.
//
// A map with string values treats an occurrence containing a single "=" as
// one pair, so its value may contain commas:
//
//	--label=tags=a,b,c
//
// yields map[string]string{"tags": "a,b,c"}.
type mapValue struct {
	dst      reflect.Value
	elem     codec.Codec
	replaced bool // the default has been cleared by a parsed occurrence
}

func newMapValue(dst reflect.Value) *mapValue {
	t := dst.Type()
	if t.Key().Kind() != reflect.String {
		panic(fmt.Sprintf("gpflag: map keys must be strings, not %s", t.Key()))
	}
	elem, ok := codecFor(t.Elem())
	if !ok {
		panic(fmt.Sprintf("gpflag: unsupported map element type %s", t.Elem()))
	}
	return &mapValue{dst: dst, elem: elem}
}

func (v *mapValue) Type() string        { return "stringTo" + upperFirst(v.elem.TypeName) }
func (v *mapValue) zeroDefault() string { return "[]" }

func (v *mapValue) Set(s string) error {
	pairs, err := splitPairs(s, v.dst.Type().Elem().Kind() == reflect.String)
	if err != nil {
		return err
	}
	parsed := reflect.MakeMapWithSize(v.dst.Type(), len(pairs))
	for _, pair := range pairs {
		key, text, ok := strings.Cut(pair, "=")
		if !ok {
			return fmt.Errorf("%s must be formatted as key=value", pair)
		}
		if v.elem.TrimSpace {
			text = strings.TrimSpace(text)
		}
		value, err := v.elem.Parse(text)
		if err != nil {
			return err
		}
		parsed.SetMapIndex(reflect.ValueOf(key).Convert(v.dst.Type().Key()), value)
	}

	// Update in place, so maps obtained through Get observe parsing.
	if v.dst.IsNil() {
		v.dst.Set(reflect.MakeMapWithSize(v.dst.Type(), parsed.Len()))
	}
	if !v.replaced {
		v.dst.Clear()
	}
	for iter := parsed.MapRange(); iter.Next(); {
		v.dst.SetMapIndex(iter.Key(), iter.Value())
	}
	v.replaced = true
	return nil
}

// String tolerates the zero value, which the standard flag package creates.
func (v *mapValue) String() string {
	if !v.dst.IsValid() {
		return ""
	}
	keys := v.dst.MapKeys()
	slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
	pairs := make([]string, len(keys))
	for i, key := range keys {
		pairs[i] = key.String() + "=" + v.elem.Format(v.dst.MapIndex(key))
	}
	return "[" + csv.Format(pairs) + "]"
}

func (v *mapValue) Get() any { return v.dst.Interface() }

// splitPairs splits s into key=value pairs. It preserves pflag's single string
// assignment rule: for string values, a=b,c is one pair whose value contains a
// comma, not an incomplete second pair.
func splitPairs(s string, stringValues bool) ([]string, error) {
	if stringValues && strings.Count(s, "=") == 1 {
		return []string{strings.Trim(s, `"`)}, nil
	}
	return csv.Read(s)
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}
