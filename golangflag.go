package gpflag

import (
	goflag "flag"
	"reflect"
	"strings"
	"time"
)

func isGotestFlag(flag string) bool          { return strings.HasPrefix(flag, "-test.") }
func isGotestShorthandFlag(flag string) bool { return strings.HasPrefix(flag, "test.") }

// Imported values have no trustworthy registered Go type for typed Lookup.
type importedFlag struct {
	inner innerflag
	value goflag.Value
}

func newImportedCore(fs *FlagSet, meta FlagMeta, value goflag.Value) *innerflag {
	f := &importedFlag{inner: innerflag{FlagMeta: meta, fs: fs}, value: value}
	f.inner.flagOps = f
	return &f.inner
}
func (f *importedFlag) setText(text string) error { return f.value.Set(text) }
func (f *importedFlag) text() string              { return f.value.String() }
func (f *importedFlag) cloneFor(fs *FlagSet) *innerflag {
	clone := *f
	clone.inner.FlagMeta = copyFlagMeta(f.inner.FlagMeta)
	clone.inner.fs, clone.inner.Changed = fs, false
	clone.inner.flagOps = &clone
	return &clone.inner
}

// AddGoFlag imports a standard-library flag as an opaque registration. Read its
// value using the original bound variable rather than typed Lookup.
func (fs *FlagSet) AddGoFlag(flag *goflag.Flag) {
	if fs.lookupCore(flag.Name) != nil {
		return
	}
	t := reflect.TypeOf(flag.Value)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	meta := FlagMeta{Name: flag.Name, Usage: flag.Usage, DefValue: flag.DefValue, Type: strings.TrimSuffix(t.Name(), "Value")}
	meta.zeroDefault = meta.DefValue == "" || meta.DefValue == "0" || meta.DefValue == "false" || meta.DefValue == "0s"
	if len(meta.Name) == 1 {
		meta.Shorthand = meta.Name
	}
	if value, ok := flag.Value.(interface{ IsBoolFlag() bool }); ok && value.IsBoolFlag() {
		meta.boolFlag, meta.NoOptDefVal = true, "true"
	}
	fs.addFlag(newImportedCore(fs, meta, flag.Value))
}

// AddGoFlagSet imports a standard-library set without guessing Go value types.
func (fs *FlagSet) AddGoFlagSet(source *goflag.FlagSet) {
	if source == nil {
		return
	}
	source.VisitAll(fs.AddGoFlag)
	fs.addedGoFlagSets = append(fs.addedGoFlagSets, source)
}

type goFlagAdapter struct{ flag *innerflag }

func (a *goFlagAdapter) String() string {
	if a.flag == nil {
		return ""
	}
	return a.flag.text()
}
func (a *goFlagAdapter) Set(text string) error { return a.flag.fs.SetText(a.flag.Name, text) }
func (a *goFlagAdapter) IsBoolFlag() bool      { return a.flag != nil && a.flag.boolFlag }

// CopyToGoFlagSet shares bound variables with a standard-library set. Native
// registrations preserve Go's help formatting. Native writes bypass this set's
// assignment tracking, just like direct writes to a bound variable.
func (fs *FlagSet) CopyToGoFlagSet(dest *goflag.FlagSet) {
	fs.visitAll(func(flag *innerflag) {
		usage := flag.Usage
		if flag.Deprecated != "" {
			usage += " (DEPRECATED: " + flag.Deprecated + ")"
		}
		if !flag.native {
			dest.Var(&goFlagAdapter{flag: flag}, flag.Name, usage)
			return
		}
		switch f := flag.flagOps.(type) {
		case *Flag[string]:
			dest.StringVar(f.value, flag.Name, f.Get(), usage)
		case *Flag[bool]:
			dest.BoolVar(f.value, flag.Name, f.Get(), usage)
		case *Flag[int]:
			dest.IntVar(f.value, flag.Name, f.Get(), usage)
		case *Flag[int64]:
			dest.Int64Var(f.value, flag.Name, f.Get(), usage)
		case *Flag[uint]:
			dest.UintVar(f.value, flag.Name, f.Get(), usage)
		case *Flag[uint64]:
			dest.Uint64Var(f.value, flag.Name, f.Get(), usage)
		case *Flag[float64]:
			dest.Float64Var(f.value, flag.Name, f.Get(), usage)
		case *Flag[time.Duration]:
			dest.DurationVar(f.value, flag.Name, f.Get(), usage)
		default:
			dest.Var(&goFlagAdapter{flag: flag}, flag.Name, usage)
		}
	})
}

// ParseSkippedFlags parses the -test.* arguments deliberately skipped by Parse.
func ParseSkippedFlags(args []string, flags *goflag.FlagSet) error {
	var skipped []string
	for _, arg := range args {
		if isGotestFlag(arg) {
			skipped = append(skipped, arg)
		}
	}
	return flags.Parse(skipped)
}
