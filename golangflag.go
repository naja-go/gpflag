// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	goflag "flag"
	"reflect"
	"strings"
	"time"
)

// go test flags prefixes
func isGotestFlag(flag string) bool {
	return strings.HasPrefix(flag, "-test.")
}

func isGotestShorthandFlag(flag string) bool {
	return strings.HasPrefix(flag, "test.")
}

// flagValueWrapper implements gpflag.Value around a flag.Value.  The main
// difference here is the addition of the Type method that returns a string
// name of the type.  As this is generally unknown, we approximate that with
// reflection.
type flagValueWrapper struct {
	inner    goflag.Value
	flagType string
}

// We are just copying the boolFlag interface out of goflag as that is what
// they use to decide if a flag should get "true" when no arg is given.
type goBoolFlag interface {
	goflag.Value
	IsBoolFlag() bool
}

func wrapFlagValue(v goflag.Value) Value {
	// If the flag.Value happens to also be a gpflag.Value, just use it directly.
	if pv, ok := v.(Value); ok {
		return pv
	}

	pv := &flagValueWrapper{
		inner: v,
	}

	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Interface || t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	pv.flagType = strings.TrimSuffix(t.Name(), "Value")
	return pv
}

func (v *flagValueWrapper) String() string {
	if v == nil || v.inner == nil {
		return ""
	}
	return v.inner.String()
}

func (v *flagValueWrapper) Get() any {
	if getter, ok := v.inner.(goflag.Getter); ok {
		return getter.Get()
	}
	return nil
}

func (v *flagValueWrapper) IsBoolFlag() bool {
	b, ok := v.inner.(goBoolFlag)
	return ok && b.IsBoolFlag()
}

func (v *flagValueWrapper) Set(s string) error {
	return v.inner.Set(s)
}

func (v *flagValueWrapper) Type() string {
	return v.flagType
}

// FlagFromGoFlag will return a *gpflag.Flag given a *flag.Flag
// If the *flag.Flag.Name was a single character (ex: `v`) it will be accessible
// with both `-v` and `--v` in flags. If the golang flag was more than a single
// character (ex: `verbose`) it will only be accessible via `--verbose`
func FlagFromGoFlag(goflag *goflag.Flag) *Flag {
	// Remember the default value as a string; it won't change.
	flag := &Flag{
		Name:  goflag.Name,
		Usage: goflag.Usage,
		Value: wrapFlagValue(goflag.Value),
		// Looks like golang flags don't set DefValue correctly  :-(
		// DefValue: goflag.DefValue,
		DefValue: goflag.Value.String(),
	}
	// Ex: if the golang flag was -v, allow both -v and --v to work
	if len(flag.Name) == 1 {
		flag.Shorthand = flag.Name
	}
	if fv, ok := goflag.Value.(goBoolFlag); ok && fv.IsBoolFlag() {
		flag.NoOptDefVal = "true"
	}
	return flag
}

// AddGoFlag will add the given *flag.Flag to the gpflag.FlagSet
func (f *FlagSet) AddGoFlag(goflag *goflag.Flag) {
	if f.Lookup(goflag.Name) != nil {
		return
	}
	newflag := FlagFromGoFlag(goflag)
	f.AddFlag(newflag)
}

// AddGoFlagSet will add the given *flag.FlagSet to the gpflag.FlagSet
func (f *FlagSet) AddGoFlagSet(newSet *goflag.FlagSet) {
	if newSet == nil {
		return
	}
	newSet.VisitAll(func(goflag *goflag.Flag) {
		f.AddGoFlag(goflag)
	})
	if f.addedGoFlagSets == nil {
		f.addedGoFlagSets = make([]*goflag.FlagSet, 0)
	}
	f.addedGoFlagSets = append(f.addedGoFlagSets, newSet)
}

// CopyToGoFlagSet will add all current flags to the given Go flag set.
// Deprecation remarks get copied into the usage description.
// Values are shared. Native scalar bindings also preserve standard help types.
//
// The standard library calls Value.Set directly, so a [FirstSetter] parsed
// through newSet does not see SetFirst: a types.StringArray appends to its
// default instead of replacing it.
func (f *FlagSet) CopyToGoFlagSet(newSet *goflag.FlagSet) {
	f.VisitAll(func(flag *Flag) {
		usage := flag.Usage
		if flag.Deprecated != "" {
			usage += " (DEPRECATED: " + flag.Deprecated + ")"
		}

		var pointer any
		if p, ok := flag.Value.(pointerer); ok {
			pointer = p.pointer()
		}
		// Delegate native bindings to flag itself, rather than reproducing its
		// formatting or zero-default detection. Only ordinary Go pointers cross
		// this boundary; custom parsers are always kept intact.
		switch p := pointer.(type) {
		case *bool:
			newSet.BoolVar(p, flag.Name, *p, usage)
		case *string:
			newSet.StringVar(p, flag.Name, *p, usage)
		case *int:
			newSet.IntVar(p, flag.Name, *p, usage)
		case *int64:
			newSet.Int64Var(p, flag.Name, *p, usage)
		case *uint:
			newSet.UintVar(p, flag.Name, *p, usage)
		case *uint64:
			newSet.Uint64Var(p, flag.Name, *p, usage)
		case *float64:
			newSet.Float64Var(p, flag.Name, *p, usage)
		case *time.Duration:
			newSet.DurationVar(p, flag.Name, *p, usage)
		default:
			newSet.Var(flag.Value, flag.Name, usage)
		}
	})
}

// ParseSkippedFlags explicitly Parses go test flags (i.e. the one starting with '-test.') with goflag.Parse(),
// since by default those are skipped by FlagSet.Parse.
// Typical usage example: `ParseSkippedFlags(os.Args[1:], goflag.CommandLine)`
func ParseSkippedFlags(osArgs []string, goFlagSet *goflag.FlagSet) error {
	var skippedFlags []string
	for _, f := range osArgs {
		if isGotestFlag(f) {
			skippedFlags = append(skippedFlags, f)
		}
	}
	return goFlagSet.Parse(skippedFlags)
}
