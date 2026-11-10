// Copyright 2026 The naja-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"reflect"
)

// newFlag constructs a new flag. This function panics if the provided p
// pointer to T is nil as this is a programmer error.
func newFlag[T any](p *T, name, shorthand string, value T, usage string) *Flag {
	if p == nil {
		panic("gpflag: nil flag destination")
	}
	dst := reflect.ValueOf(p).Elem()

	// A T that is itself a Value, such as a pointer the caller already holds,
	// is used directly. Otherwise validate support before changing dst.
	v, ok := valueItself(value)
	if !ok {
		v = eraseValue(dst)
	}

	dst.Set(cloneCollections(reflect.ValueOf(&value).Elem()))
	return &Flag{
		Name: name, Shorthand: shorthand, Usage: usage,
		Value: v, DefValue: v.String(),
	}
}

// valueItself reports whether value, a pointer or interface, is a usable Value.
// This function panics if the provided value is a nil pointer - this is a programmer error.
func valueItself[T any](value T) (Value, bool) {
	v, ok := any(value).(Value)
	if !ok {
		return nil, false
	}
	switch reflect.TypeFor[T]().Kind() {
	case reflect.Pointer, reflect.Interface:
	default:
		return nil, false // *T is the Value; eraseValue binds it to the destination.
	}
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		panic(fmt.Sprintf("gpflag: nil %T flag value", value))
	}
	return v, true
}

// New constructs and registers a flag in f, returning a pointer to its value.
// The pointer holds the default until Parse stores a parsed value in it.
//
// It accepts native scalars (including named types), slices, string-keyed maps,
// standard-library text values, and custom Values: either a type whose pointer
// implements Value, or a non-nil pointer or interface that is itself a Value.
// Unsupported types panic at construction time.
//
// To configure the registered Flag, use NewVar, which returns it, or Lookup.
func (f *FlagSet) New[T any](name string, value T, usage string) *T {
	return f.NewP(name, "", value, usage)
}

// NewP is like New, with a one-letter shorthand.
func (f *FlagSet) NewP[T any](name, shorthand string, value T, usage string) *T {
	p := new(T)
	f.NewVarP(p, name, shorthand, value, usage)
	return p
}

// NewVar constructs and registers a flag in f bound to p, initialized to value,
// and returns the Flag so that it can be configured further.
// Slice and map defaults are copied so parsing does not modify the defaults.
func (f *FlagSet) NewVar[T any](p *T, name string, value T, usage string) *Flag {
	return f.NewVarP(p, name, "", value, usage)
}

// NewVarP is like NewVar, with a one-letter shorthand. A nil p panics.
func (f *FlagSet) NewVarP[T any](p *T, name, shorthand string, value T, usage string) *Flag {
	flag := newFlag(p, name, shorthand, value, usage)
	f.AddFlag(flag)
	return flag
}

// Get returns the flag's stored value as T, without a text round-trip. Slices
// and maps share storage with the flag, as ordinary Go values do.
//
// T must be the flag's exact type: the type passed to New, including any named
// type. Get panics on a mismatch, since that is a programming error; use TryGet
// when the flag or its type is not known statically.
func Get[T any](flag *Flag) T {
	value, err := TryGet[T](flag)
	if err != nil {
		panic(err)
	}
	return value
}

// TryGet is like Get, but returns an error for nil flags and mismatched types.
//
// T matches when the flag's Value is a *T (a custom Value registered by value),
// when its Getter returns a T (every native flag built by New), or when the
// Value itself is a T (a custom Value registered by pointer).
func TryGet[T any](flag *Flag) (T, error) {
	var zero T
	if flag == nil {
		return zero, fmt.Errorf("gpflag: cannot get a value from a nil flag")
	}
	if p, ok := any(flag.Value).(*T); ok && p != nil {
		return *p, nil
	}
	if getter, ok := flag.Value.(Getter); ok {
		value := getter.Get()
		if result, ok := value.(T); ok {
			return result, nil
		}
		return zero, fmt.Errorf("gpflag: flag %q has type %T, not %s", flag.Name, value, reflect.TypeFor[T]())
	}
	if result, ok := any(flag.Value).(T); ok {
		return result, nil
	}
	return zero, fmt.Errorf("gpflag: flag %q has value type %T, not %s", flag.Name, flag.Value, reflect.TypeFor[T]())
}

// Get returns the flag's value as T. It panics if T is not the flag's type.
func (f *Flag) Get[T any]() T { return Get[T](f) }

// TryGet returns the flag's value as T, or an error if T is not the flag's type.
func (f *Flag) TryGet[T any]() (T, error) { return TryGet[T](f) }

// Get looks up a flag by name and returns its value as T. It panics if the flag
// does not exist or T is not its type.
func (f *FlagSet) Get[T any](name string) T {
	value, err := f.TryGet[T](name)
	if err != nil {
		panic(err)
	}
	return value
}

// TryGet looks up a flag by name and returns its value as T.
func (f *FlagSet) TryGet[T any](name string) (T, error) {
	flag := f.Lookup(name)
	if flag == nil {
		var zero T
		return zero, &NotExistError{name: name, messageType: flagNotExistMessage}
	}
	return TryGet[T](flag)
}

// Copy collection storage, including collections inside structs such as net.IPNet.
// Other fields keep normal Go assignment semantics; arbitrary object graphs are
// not deep-copied.
func cloneCollections(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			out.Index(i).Set(cloneCollections(v.Index(i)))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), cloneCollections(iter.Value()))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				out.Field(i).Set(cloneCollections(v.Field(i)))
			}
		}
		return out
	default:
		return v
	}
}
