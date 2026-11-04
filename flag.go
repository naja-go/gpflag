// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package gpflag implements generic flags with POSIX/GNU command-line syntax.
It is a hard fork of pflag, not an API-compatible replacement.

New, NewP, NewVar, and NewVarP register the actual generic Flag[T]:

	port := gpflag.NewP("port", "p", 8080, "listen port").Env("APP_PORT")
	tags := gpflag.New[[]string]("tag", nil, "tags")
	gpflag.Parse()
	fmt.Println(port.Get(), tags.Get())

A FlagSet provides the same constructors for subcommands and independent
parsers. Lookup[T] retrieves the registered object by exact Go type. Flag.Set
assigns a Go value; Flag.SetText and FlagSet.SetText parse argument text.

Typed options select syntax: Count applies to int, StringArray to []string,
Base64 to []byte, and TimeFormats to time.Time. Custom types can implement
encoding.TextUnmarshaler or supply WithParser. There is no untyped Value API.
VisitAll and Visit expose isolated FlagMeta snapshots, not parsers or values.

Command-line syntax includes --flag=value, --flag value, single-dash shorthand
clusters, interspersed positional arguments, and the -- terminator. See the
README for supported types, ownership, and environment fallback rules.
*/
package gpflag

import (
	"bytes"
	"errors"
	goflag "flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ErrHelp is the error returned if the flag -help is invoked but no such flag is defined.
var ErrHelp = errors.New("gpflag: help requested")

// ErrorHandling defines how to handle flag parsing errors.
type ErrorHandling int

const (
	// ContinueOnError will return an err from Parse() if an error is found
	ContinueOnError ErrorHandling = iota
	// ExitOnError will call os.Exit(2) if an error is found when parsing
	ExitOnError
	// PanicOnError will panic() if an error is found when parsing flags
	PanicOnError
)

// UnknownFlagsHandling decides how to handle unknown flags
type UnknownFlagsHandling int

const (
	// ErrorOnUnknownFlag will return an error if an unknown flag is found
	ErrorOnUnknownFlag UnknownFlagsHandling = iota
	// IgnoreUnknownFlag will ignore unknown flags and continue parsing rest of the flags
	IgnoreUnknownFlag
	// PassUnknownFlagToArgs will treat unknown flags as non-flag arguments.
	// Combined shorthand flags mixed with known ones and unknown ones results
	// combined flags only with unknown ones.
	// E.g. -fghi results -gh if only `f` and `i` are known.
	PassUnknownFlagToArgs
)

// ParseErrorsAllowlist defines the parsing errors that can be ignored
type ParseErrorsAllowlist struct {
	// UnknownFlags will ignore unknown flags errors and continue parsing rest of the flags
	// Deprecated: Use UnknownFlagsHandling instead
	UnknownFlags bool

	// UnknownFlagsHandling decides how to handle unknown flags. Defaults to UnknownFlagsHandlingErrorOnUnknown.
	UnknownFlagsHandling UnknownFlagsHandling
}

// ParseErrorsWhitelist defines the parsing errors that can be ignored.
//
// Deprecated: use [ParseErrorsAllowlist] instead. This type will be removed in a future release.
type ParseErrorsWhitelist = ParseErrorsAllowlist

// NormalizedName is a flag name that has been normalized according to rules
// for the FlagSet (e.g. making '-' and '_' equivalent).
type NormalizedName string

// A FlagSet represents a set of defined flags.
type FlagSet struct {
	// Usage is the function called when an error occurs while parsing flags.
	// The field is a function (not a method) that may be changed to point to
	// a custom error handler.
	Usage func()

	// SortFlags is used to indicate, if user wants to have sorted flags in
	// help/usage messages.
	SortFlags bool

	// ParseErrorsAllowlist is used to configure an allowlist of errors
	ParseErrorsAllowlist ParseErrorsAllowlist

	// ParseErrorsAllowlist is used to configure an allowlist of errors.
	//
	// Deprecated: use [FlagSet.ParseErrorsAllowlist] instead. This field will be removed in a future release.
	ParseErrorsWhitelist ParseErrorsAllowlist

	name              string
	parsed            bool
	actual            map[NormalizedName]*innerflag
	orderedActual     []*innerflag
	sortedActual      []*innerflag
	formal            map[NormalizedName]*innerflag
	orderedFormal     []*innerflag
	sortedFormal      []*innerflag
	shorthands        map[byte]*innerflag
	args              []string // arguments after flags
	argsLenAtDash     int      // len(args) when a '--' was located when parsing, or -1 if no --
	errorHandling     ErrorHandling
	output            io.Writer // nil means stderr; use Output() accessor
	interspersed      bool      // allow interspersed option/non-option args
	normalizeNameFunc func(f *FlagSet, name string) NormalizedName

	addedGoFlagSets []*goflag.FlagSet
}

// Flag is a registered flag with a Go value of type T. Construct it with New,
// NewP, NewVar, or NewVarP. A flag belongs to one FlagSet and must not be copied.
// Get returns the Go value without copying slice, map, or pointer storage.
type Flag[T any] struct {
	inner innerflag
	value *T
	codec codec[T]
}

// sortFlags returns the flags as a slice in lexicographical sorted order.
func sortFlags(flags map[NormalizedName]*innerflag) []*innerflag {
	list := make(sort.StringSlice, len(flags))
	i := 0
	for k := range flags {
		list[i] = string(k)
		i++
	}
	list.Sort()
	result := make([]*innerflag, len(list))
	for i, name := range list {
		result[i] = flags[NormalizedName(name)]
	}
	return result
}

// SetNormalizeFunc allows you to add a function which can translate flag names.
// Flags added to the FlagSet will be translated and then when anything tries to
// look up the flag that will also be translated. So it would be possible to create
// a flag named "getURL" and have it translated to "geturl".  A user could then pass
// "--getUrl" which may also be translated to "geturl" and everything will work.
func (f *FlagSet) SetNormalizeFunc(n func(f *FlagSet, name string) NormalizedName) {
	formal := make(map[NormalizedName]*innerflag, len(f.formal))
	actual := make(map[NormalizedName]*innerflag, len(f.actual))
	for oldName, flag := range f.formal {
		name := NormalizedName(flag.Name)
		if n != nil {
			name = n(f, string(name))
		}
		if previous := formal[name]; previous != nil {
			panic(fmt.Sprintf("gpflag: normalization makes flags %q and %q collide", previous.Name, flag.Name))
		}
		formal[name] = flag
		if f.actual[oldName] != nil {
			actual[name] = flag
		}
	}
	// Publish only after every name has been validated.
	for name, flag := range formal {
		flag.Name = string(name)
	}
	f.normalizeNameFunc, f.formal, f.actual = n, formal, actual
	f.sortedFormal, f.sortedActual = nil, nil
}

// GetNormalizeFunc returns the previously set NormalizeFunc of a function which
// does no translation, if not set previously.
func (f *FlagSet) GetNormalizeFunc() func(f *FlagSet, name string) NormalizedName {
	if f.normalizeNameFunc != nil {
		return f.normalizeNameFunc
	}
	return func(_ *FlagSet, name string) NormalizedName { return NormalizedName(name) }
}

func (f *FlagSet) normalizeFlagName(name string) NormalizedName {
	n := f.GetNormalizeFunc()
	return n(f, name)
}

// Output returns the destination for usage and error messages. os.Stderr is returned if
// output was not set or was set to nil.
func (f *FlagSet) Output() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

// Name returns the name of the flag set.
func (f *FlagSet) Name() string {
	return f.name
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
func (f *FlagSet) SetOutput(output io.Writer) {
	f.output = output
}

// visitAll visits registrations in the set's configured order, without snapshots.
func (f *FlagSet) visitAll(fn func(*innerflag)) {
	if len(f.formal) == 0 {
		return
	}

	var flags []*innerflag
	if f.SortFlags {
		if len(f.formal) != len(f.sortedFormal) {
			f.sortedFormal = sortFlags(f.formal)
		}
		flags = f.sortedFormal
	} else {
		flags = f.orderedFormal
	}

	for _, flag := range flags {
		fn(flag)
	}
}

// VisitAll visits isolated metadata snapshots in the set's configured order.
func (f *FlagSet) VisitAll(fn func(FlagMeta)) {
	f.visitAll(func(flag *innerflag) { fn(copyFlagMeta(flag.FlagMeta)) })
}

// HasFlags returns a bool to indicate if the FlagSet has any flags defined.
func (f *FlagSet) HasFlags() bool {
	return len(f.formal) > 0
}

// HasAvailableFlags returns a bool to indicate if the FlagSet has any flags
// that are not hidden.
func (f *FlagSet) HasAvailableFlags() bool {
	for _, flag := range f.formal {
		if !flag.Hidden {
			return true
		}
	}
	return false
}

// getUnknownFlagsHandling returns the UnknownFlagsHandling value,
// considering deprecated ParseErrorsWhitelist and UnknownFlags field
// After removing ParseErrorsWhitelist, this function can be simplified
// and moved to ParseErrorsAllowlist.getUnknownFlagsHandling()
func (f *FlagSet) getUnknownFlagsHandling() UnknownFlagsHandling {
	// first check ParseErrorsAllowlist:
	// if UnknownFlagsHandling is set, use it
	if f.ParseErrorsAllowlist.UnknownFlagsHandling != ErrorOnUnknownFlag {
		return f.ParseErrorsAllowlist.UnknownFlagsHandling
	}

	if f.ParseErrorsAllowlist.UnknownFlags {
		return IgnoreUnknownFlag
	}

	// then, check deprecated ParseErrorsWhitelist:
	// if UnknownFlagsHandling is set, use it
	if f.ParseErrorsWhitelist.UnknownFlagsHandling != ErrorOnUnknownFlag {
		return f.ParseErrorsWhitelist.UnknownFlagsHandling
	}

	if f.ParseErrorsWhitelist.UnknownFlags {
		return IgnoreUnknownFlag
	}

	// Otherwise, return the default value
	return ErrorOnUnknownFlag
}

// VisitAll visits the command-line flags in lexicographical order or
// in primordial order if f.SortFlags is false, calling fn for each.
// It visits all flags, even those not set.
func VisitAll(fn func(FlagMeta)) {
	CommandLine.VisitAll(fn)
}

// Visit visits the flags in lexicographical order or
// in primordial order if f.SortFlags is false, calling fn for each.
// It visits only those flags that have been set.
func (f *FlagSet) Visit(fn func(FlagMeta)) {
	if len(f.actual) == 0 {
		return
	}

	var flags []*innerflag
	if f.SortFlags {
		if len(f.actual) != len(f.sortedActual) {
			f.sortedActual = sortFlags(f.actual)
		}
		flags = f.sortedActual
	} else {
		flags = f.orderedActual
	}

	for _, flag := range flags {
		fn(copyFlagMeta(flag.FlagMeta))
	}
}

// Visit visits the command-line flags in lexicographical order or
// in primordial order if f.SortFlags is false, calling fn for each.
// It visits only those flags that have been set.
func Visit(fn func(FlagMeta)) {
	CommandLine.Visit(fn)
}

// lookupCore returns the named registration, or nil if it is missing.
func (f *FlagSet) lookupCore(name string) *innerflag {
	return f.formal[f.normalizeFlagName(name)]
}

// ShorthandLookup returns the metadata snapshot of the short handed flag,
// returning nil if none exists.
// It panics, if len(name) > 1.
func (f *FlagSet) ShorthandLookup(name string) *FlagMeta {
	if len(name) > 1 {
		panic(fmt.Sprintf("can not look up shorthand which is more than one ASCII character: %q", name))
	}
	if name != "" {
		if flag := f.shorthands[name[0]]; flag != nil {
			snapshot := copyFlagMeta(flag.FlagMeta)
			return &snapshot
		}
	}
	return nil
}

// ArgsLenAtDash will return the length of f.Args at the moment when a -- was
// found during arg parsing. This allows your program to know which args were
// before the -- and which came after.
func (f *FlagSet) ArgsLenAtDash() int {
	return f.argsLenAtDash
}

// MarkDeprecated indicated that a flag is deprecated in your program. It will
// continue to function but will not show up in help or usage messages. Using
// this flag will also print the given usageMessage.
func (f *FlagSet) MarkDeprecated(name string, usageMessage string) error {
	flag := f.lookupCore(name)
	if flag == nil {
		return &NotExistError{name: name, messageType: flagNotExistMessage}
	}
	if usageMessage == "" {
		return fmt.Errorf("deprecated message for flag %q must be set", name)
	}
	flag.Deprecated = usageMessage
	flag.Hidden = true
	return nil
}

// MarkShorthandDeprecated will mark the shorthand of a flag deprecated in your
// program. It will continue to function but will not show up in help or usage
// messages. Using this flag will also print the given usageMessage.
func (f *FlagSet) MarkShorthandDeprecated(name string, usageMessage string) error {
	flag := f.lookupCore(name)
	if flag == nil {
		return &NotExistError{name: name, messageType: flagNotExistMessage}
	}
	if usageMessage == "" {
		return fmt.Errorf("deprecated message for flag %q must be set", name)
	}
	flag.ShorthandDeprecated = usageMessage
	return nil
}

// MarkHidden sets a flag to 'hidden' in your program. It will continue to
// function but will not show up in help or usage messages.
func (f *FlagSet) MarkHidden(name string) error {
	flag := f.lookupCore(name)
	if flag == nil {
		return &NotExistError{name: name, messageType: flagNotExistMessage}
	}
	flag.Hidden = true
	return nil
}

// ShorthandLookup returns the metadata snapshot of the short handed flag,
// returning nil if none exists.
func ShorthandLookup(name string) *FlagMeta {
	return CommandLine.ShorthandLookup(name)
}

// SetText parses text for the named flag. For a Go value, use Flag.Set.
func (f *FlagSet) SetText(name, value string) error {
	flag := f.formal[f.normalizeFlagName(name)]
	if flag == nil {
		return &NotExistError{name: name, messageType: flagNoSuchFlagMessage}
	}
	if err := flag.setText(value); err != nil {
		return &InvalidValueError{flag: copyFlagMeta(flag.FlagMeta), value: value, cause: err}
	}
	f.recordChanged(flag)
	if message := flag.Deprecated; message != "" {
		_, _ = fmt.Fprintf(f.Output(), "Flag --%s has been deprecated, %s\n", name, message)
	}
	return nil
}

// SetAnnotation allows one to set arbitrary annotations on a flag in the FlagSet.
// This is sometimes used by spf13/cobra programs which want to generate additional
// bash completion information.
func (f *FlagSet) SetAnnotation(name, key string, values []string) error {
	normalName := f.normalizeFlagName(name)
	flag, ok := f.formal[normalName]
	if !ok {
		return &NotExistError{name: name, messageType: flagNoSuchFlagMessage}
	}
	if flag.Annotations == nil {
		flag.Annotations = map[string][]string{}
	}
	flag.Annotations[key] = append([]string(nil), values...)
	return nil
}

// Changed returns true if the flag was explicitly set during Parse() and false
// otherwise
func (f *FlagSet) Changed(name string) bool {
	flag := f.lookupCore(name)
	// If a flag doesn't exist, it wasn't changed....
	if flag == nil {
		return false
	}
	return flag.Changed
}

// SetText parses text for the named command-line flag.
func SetText(name, value string) error {
	return CommandLine.SetText(name, value)
}

// PrintDefaults prints, to standard error unless configured
// otherwise, the default values of all defined flags in the set.
func (f *FlagSet) PrintDefaults() {
	usages := f.FlagUsages()
	_, _ = fmt.Fprint(f.Output(), usages)
}

// defaultIsZeroValue returns true if the default value for this flag represents
// a zero value.
func (f FlagMeta) defaultIsZeroValue() bool { return f.zeroDefault }

// UnquoteUsage extracts a back-quoted name from the usage
// string for a flag and returns it and the un-quoted usage.
// Given "a `name` to show" it returns ("name", "a name to show").
// If there are no back quotes, the name is an educated guess of the
// type of the flag's value, or the empty string if the flag is boolean.
func UnquoteUsage(flag FlagMeta) (name string, usage string) {
	// Look for a back-quoted name, but avoid the strings package.
	usage = flag.Usage
	for i := 0; i < len(usage); i++ {
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				if usage[j] == '`' {
					name = usage[i+1 : j]
					usage = usage[:i] + name + usage[j+1:]
					return name, usage
				}
			}
			break // Only one back quote; use type name.
		}
	}

	name = flag.Type
	if flag.boolFlag {
		name = ""
	}
	switch name {
	case "bool", "boolfunc":
		name = ""
	case "func":
		name = "value"
	case "float64":
		name = "float"
	case "int64":
		name = "int"
	case "uint64":
		name = "uint"
	case "stringSlice":
		name = "strings"
	case "intSlice":
		name = "ints"
	case "uintSlice":
		name = "uints"
	case "boolSlice":
		name = "bools"
	}

	return
}

// Splits the string `s` on whitespace into an initial substring up to
// `i` runes in length and the remainder. Will go `slop` over `i` if
// that encompasses the entire string (which allows the caller to
// avoid short orphan words on the final line). If the next word is
// wider than `i`, it is returned on its own and wrapping continues
// after it.
func wrapN(i, slop int, s string) (string, string) {
	if i+slop > len(s) {
		return s, ""
	}

	w := strings.LastIndexAny(s[:i], " \t\n")
	if w <= 0 {
		// No break in the first i bytes. Emit the over-wide word
		// alone and keep wrapping whatever follows.
		next := strings.IndexAny(s, " \t\n")
		if next <= 0 {
			return s, ""
		}
		return s[:next], s[next+1:]
	}
	nlPos := strings.LastIndex(s[:i], "\n")
	if nlPos > 0 && nlPos < w {
		return s[:nlPos], s[nlPos+1:]
	}
	return s[:w], s[w+1:]
}

// Wraps the string `s` to a maximum width `w` with leading indent
// `i`. The first line is not indented (this is assumed to be done by
// caller). Pass `w` == 0 to do no wrapping
func wrap(i, w int, s string) string {
	if w == 0 {
		return strings.ReplaceAll(s, "\n", "\n"+strings.Repeat(" ", i))
	}

	// space between indent i and end of line width w into which
	// we should wrap the text.
	wrap := w - i

	var r, l string

	// Not enough space for sensible wrapping. Wrap as a block on
	// the next line instead.
	if wrap < 24 {
		i = 16
		wrap = w - i
		r += "\n" + strings.Repeat(" ", i)
	}
	// If still not enough space then don't even try to wrap.
	if wrap < 24 {
		return strings.ReplaceAll(s, "\n", r)
	}

	// Try to avoid short orphan words on the final line, by
	// allowing wrapN to go a bit over if that would fit in the
	// remainder of the line.
	slop := 5
	wrap = wrap - slop

	// Handle first line, which is indented by the caller (or the
	// special case above)
	l, s = wrapN(wrap, slop, s)
	r = r + strings.ReplaceAll(l, "\n", "\n"+strings.Repeat(" ", i))

	// Now wrap the rest
	for s != "" {
		var t string

		t, s = wrapN(wrap, slop, s)
		r = r + "\n" + strings.Repeat(" ", i) + strings.ReplaceAll(t, "\n", "\n"+strings.Repeat(" ", i))
	}

	return r
}

// FlagUsagesWrapped returns a string containing the usage information
// for all flags in the FlagSet. Wrapped to `cols` columns (0 for no
// wrapping)
func (f *FlagSet) FlagUsagesWrapped(cols int) string {
	buf := new(bytes.Buffer)

	lines := make([]string, 0, len(f.formal))

	maxlen := 0
	f.visitAll(func(flag *innerflag) {
		if flag.Hidden {
			return
		}

		line := ""
		if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
			line = fmt.Sprintf("  -%s, --%s", flag.Shorthand, flag.Name)
		} else {
			line = fmt.Sprintf("      --%s", flag.Name)
		}

		varname, usage := UnquoteUsage(flag.FlagMeta)
		if flag.boolFlag && flag.Type == "bool" {
			line += "[=true|false]"
		} else if varname != "" {
			line += " " + varname
		}

		if flag.NoOptDefVal != "" {
			switch flag.Type {
			case "string":
				line += fmt.Sprintf("[=\"%s\"]", flag.NoOptDefVal)
			case "bool", "boolfunc":
				if flag.NoOptDefVal != "true" {
					line += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
				}
			case "count":
				if flag.NoOptDefVal != "+1" {
					line += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
				}
			default:
				line += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
			}
		}

		// This special character will be replaced with spacing once the
		// correct alignment is calculated
		line += "\x00"
		if len(line) > maxlen {
			maxlen = len(line)
		}

		line += usage
		if !flag.defaultIsZeroValue() {
			if flag.Type == "string" {
				line += fmt.Sprintf(" (default %q)", flag.DefValue)
			} else {
				line += fmt.Sprintf(" (default %s)", flag.DefValue)
			}
		}
		if len(flag.Deprecated) != 0 {
			line += fmt.Sprintf(" (DEPRECATED: %s)", flag.Deprecated)
		}

		lines = append(lines, line)
	})

	for _, line := range lines {
		sidx := strings.Index(line, "\x00")
		spacing := strings.Repeat(" ", maxlen-sidx)
		// maxlen + 2 comes from + 1 for the \x00 and + 1 for the (deliberate) off-by-one in maxlen-sidx
		_, _ = fmt.Fprintln(buf, line[:sidx], spacing, wrap(maxlen+2, cols, line[sidx+1:]))
	}

	return buf.String()
}

// FlagUsages returns a string containing the usage information for all flags in
// the FlagSet
func (f *FlagSet) FlagUsages() string {
	return f.FlagUsagesWrapped(0)
}

// PrintDefaults prints to standard error the default values of all defined command-line flags.
func PrintDefaults() {
	CommandLine.PrintDefaults()
}

// defaultUsage is the default function to print a usage message.
func defaultUsage(f *FlagSet) {
	_, _ = fmt.Fprintf(f.Output(), "Usage of %s:\n", f.name)
	f.PrintDefaults()
}

// NOTE: Usage is not just defaultUsage(CommandLine)
// because it serves (via godoc flag Usage) as the example
// for how to write your own usage function.

// Usage prints, to the set destination, a usage message documenting all defined
// command-line flags. The function is a variable that may be changed to point to a
// custom function. By default it prints a simple header and calls PrintDefaults; for
// details about the format of the output and how to control it, see the documentation
// for PrintDefaults.
var Usage = func() {
	_, _ = fmt.Fprintf(CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	PrintDefaults()
}

// NFlag returns the number of flags that have been set.
func (f *FlagSet) NFlag() int { return len(f.actual) }

// NFlag returns the number of command-line flags that have been set.
func NFlag() int { return len(CommandLine.actual) }

// Arg returns the i'th argument.  Arg(0) is the first remaining argument
// after flags have been processed.
func (f *FlagSet) Arg(i int) string {
	if i < 0 || i >= len(f.args) {
		return ""
	}
	return f.args[i]
}

// Arg returns the i'th command-line argument.  Arg(0) is the first remaining argument
// after flags have been processed.
func Arg(i int) string {
	return CommandLine.Arg(i)
}

// NArg is the number of arguments remaining after flags have been processed.
func (f *FlagSet) NArg() int { return len(f.args) }

// NArg is the number of arguments remaining after flags have been processed.
func NArg() int { return len(CommandLine.args) }

// Args returns the non-flag arguments.
func (f *FlagSet) Args() []string { return f.args }

// Args returns the non-flag command-line arguments.
func Args() []string { return CommandLine.args }

// addFlag validates a registration completely before publishing it.
func (f *FlagSet) addFlag(flag *innerflag) {
	name := f.normalizeFlagName(flag.Name)
	if _, exists := f.formal[name]; exists {
		panic(fmt.Sprintf("%s flag redefined: %s", f.name, flag.Name))
	}
	shorthand := flag.Shorthand
	if len(shorthand) > 1 || (len(shorthand) == 1 && shorthand[0] > 127) {
		panic(fmt.Sprintf("%q shorthand is more than one ASCII character", shorthand))
	}
	if shorthand != "" && f.shorthands[shorthand[0]] != nil {
		panic(fmt.Sprintf("unable to redefine %q shorthand in %q flagset: it's already used for %q flag", shorthand, f.name, f.shorthands[shorthand[0]].Name))
	}
	flag.Name = string(name)
	if f.formal == nil {
		f.formal = make(map[NormalizedName]*innerflag)
	}
	f.formal[name] = flag
	f.orderedFormal = append(f.orderedFormal, flag)
	f.sortedFormal = nil
	if shorthand != "" {
		if f.shorthands == nil {
			f.shorthands = make(map[byte]*innerflag)
		}
		f.shorthands[shorthand[0]] = flag
	}
}

func (f *FlagSet) setShorthand(flag *innerflag, shorthand string) {
	if len(shorthand) > 1 || (len(shorthand) == 1 && shorthand[0] > 127) {
		panic(fmt.Sprintf("%q shorthand is more than one ASCII character", shorthand))
	}
	if shorthand != "" {
		if used := f.shorthands[shorthand[0]]; used != nil && used != flag {
			panic(fmt.Sprintf("unable to redefine %q shorthand in %q flagset: it's already used for %q flag", shorthand, f.name, used.Name))
		}
	}
	if flag.Shorthand != "" {
		delete(f.shorthands, flag.Shorthand[0])
	}
	flag.Shorthand = shorthand
	if shorthand != "" {
		if f.shorthands == nil {
			f.shorthands = make(map[byte]*innerflag)
		}
		f.shorthands[shorthand[0]] = flag
	}
}

// AddFlag creates an independent registration sharing the original value's
// bound variable. Metadata and assignment tracking are not shared.
func (f *FlagSet) AddFlag[T any](flag *Flag[T]) *Flag[T] {
	clone := flag.inner.cloneFor(f)
	f.addFlag(clone)
	return clone.flagOps.(*Flag[T])
}

// AddFlagSet inherits flags not already defined in f. Registrations are cloned;
// bound variables remain shared. Each set normalizes names independently.
func (f *FlagSet) AddFlagSet(source *FlagSet) {
	if source == nil {
		return
	}
	source.visitAll(func(flag *innerflag) {
		if f.lookupCore(flag.Name) == nil {
			f.addFlag(flag.cloneFor(f))
		}
	})
}

// fail prints an error message and usage message to standard error and
// returns the error.
func (f *FlagSet) fail(err error) error {
	if f.errorHandling != ContinueOnError {
		f.usage()
	}
	return err
}

// usage calls the Usage method for the flag set, or the usage function if
// the flag set is CommandLine.
func (f *FlagSet) usage() {
	if f == CommandLine {
		Usage()
	} else if f.Usage == nil {
		defaultUsage(f)
	} else {
		f.Usage()
	}
}

// --unknown (args will be empty)
// --unknown --next-flag ... (args will be --next-flag ...)
// --unknown arg ... (args will be arg ...)
func stripUnknownFlagValue(args []string) []string {
	if len(args) == 0 {
		// --unknown
		return args
	}

	first := args[0]
	if len(first) > 0 && first[0] == '-' {
		// --unknown --next-flag ...
		return args
	}

	// --unknown arg ... (args will be arg ...)
	if len(args) > 1 {
		return args[1:]
	}
	return nil
}

// errUnknownFlag is used for internal unknown flag handling.
type unknownFlagError struct {
	// UnknownFlags is flags that are unknown and unprocessed.
	// It depends on the context whether this has a prefix like '-' or '--'.
	UnknownFlags string
}

func (e *unknownFlagError) Error() string {
	return fmt.Sprintf("unknown flag: %v", e.UnknownFlags)
}

func (f *FlagSet) parseLongArg(s string, args []string, fn parseFunc) (a []string, err error) {
	a = args
	name := s[2:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		err = f.fail(&InvalidSyntaxError{specifiedFlag: s})
		return
	}

	split := strings.SplitN(name, "=", 2)
	name = split[0]
	flag, exists := f.formal[f.normalizeFlagName(name)]
	unknownFlagsHandling := f.getUnknownFlagsHandling()

	if !exists {
		switch {
		case name == "help":
			f.usage()
			return a, ErrHelp
		case unknownFlagsHandling == IgnoreUnknownFlag:
			// --unknown=unknownval arg ...
			// we do not want to lose arg in this case
			if len(split) >= 2 {
				return a, nil
			}

			return stripUnknownFlagValue(a), nil
		case unknownFlagsHandling == PassUnknownFlagToArgs:
			return a, &unknownFlagError{
				UnknownFlags: s,
			}
		default:
			err = f.fail(&NotExistError{name: name, messageType: flagUnknownFlagMessage})
			return
		}
	}

	var value string
	if len(split) == 2 {
		// '--flag=arg'
		value = split[1]
	} else if flag.NoOptDefVal != "" {
		// '--flag' (arg was optional)
		value = flag.NoOptDefVal
	} else if len(a) > 0 {
		// '--flag arg'
		value = a[0]
		a = a[1:]
	} else {
		// '--flag' (arg was required)
		err = f.fail(&ValueRequiredError{
			flag:          copyFlagMeta(flag.FlagMeta),
			specifiedName: name,
		})
		return
	}

	// Pass the name the user actually typed so a deprecation message reports
	// it rather than the canonical name a NormalizeFunc may have mapped it to.
	err = fn(flag, name, value)
	if err != nil {
		err = f.fail(err)
	}
	return
}

func (f *FlagSet) parseSingleShortArg(shorthands string, args []string, fn parseFunc) (outShorts string, outArgs []string, err error) {
	outArgs = args

	if isGotestShorthandFlag(shorthands) {
		return
	}

	outShorts = shorthands[1:]
	c := shorthands[0]

	flag, exists := f.shorthands[c]
	if !exists {
		unknownFlagsHandling := f.getUnknownFlagsHandling()

		switch {
		case c == 'h':
			f.usage()
			err = ErrHelp
			return
		case unknownFlagsHandling == IgnoreUnknownFlag:
			// '-f=arg arg ...'
			// we do not want to lose arg in this case
			if len(shorthands) > 2 && shorthands[1] == '=' {
				outShorts = ""
				return
			}

			outArgs = stripUnknownFlagValue(outArgs)
			return
		case unknownFlagsHandling == PassUnknownFlagToArgs:
			// '-f=arg': pass all the argument
			if len(shorthands) > 2 && shorthands[1] == '=' {
				outShorts = ""
				err = &unknownFlagError{
					UnknownFlags: shorthands,
				}
				return
			}
			// '-fgh': pass only the first switch
			err = &unknownFlagError{
				UnknownFlags: shorthands[0:1],
			}
			return
		default:
			err = f.fail(&NotExistError{
				name:                string(c),
				specifiedShorthands: shorthands,
				messageType:         flagUnknownShorthandFlagMessage,
			})
			return
		}
	}

	var value string
	if len(shorthands) > 2 && shorthands[1] == '=' {
		// '-f=arg'
		value = shorthands[2:]
		outShorts = ""
	} else if flag.NoOptDefVal != "" {
		// '-f' (arg was optional)
		value = flag.NoOptDefVal
	} else if len(shorthands) > 1 {
		// '-farg'
		value = shorthands[1:]
		outShorts = ""
	} else if len(args) > 0 {
		// '-f arg'
		value = args[0]
		outArgs = args[1:]
	} else {
		// '-f' (arg was required)
		err = f.fail(&ValueRequiredError{
			flag:                copyFlagMeta(flag.FlagMeta),
			specifiedName:       string(c),
			specifiedShorthands: shorthands,
		})
		return
	}

	if flag.ShorthandDeprecated != "" {
		_, _ = fmt.Fprintf(f.Output(), "Flag shorthand -%s has been deprecated, %s\n", flag.Shorthand, flag.ShorthandDeprecated)
	}

	err = fn(flag, flag.Name, value)
	if err != nil {
		err = f.fail(err)
	}
	return
}

func (f *FlagSet) parseShortArg(s string, args []string, fn parseFunc) (a []string, err error) {
	a = args
	shorthands := s[1:]
	var errUnknownFlagAll *unknownFlagError

	// "shorthands" can be a series of shorthand letters of flags (e.g. "-vvv").
	for len(shorthands) > 0 {
		shorthands, a, err = f.parseSingleShortArg(shorthands, args, fn)
		if err != nil {
			if errUnknownFlag, ok := err.(*unknownFlagError); ok { //nolint:errorlint
				// this means f.ParseErrorsAllowlist.UnknownFlagsHandling is set to UnknownFlagsHandlingPassUnknownToArgs
				if errUnknownFlagAll == nil {
					errUnknownFlagAll = &unknownFlagError{
						UnknownFlags: "-",
					}
				}

				errUnknownFlagAll.UnknownFlags = errUnknownFlagAll.UnknownFlags +
					errUnknownFlag.UnknownFlags
				err = nil
			} else {
				return
			}
		}
	}
	if errUnknownFlagAll != nil {
		err = errUnknownFlagAll
	}

	return
}

func (f *FlagSet) parseArgs(args []string, fn parseFunc) (err error) {
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		if len(s) == 0 || s[0] != '-' || len(s) == 1 {
			if !f.interspersed {
				f.args = append(f.args, s)
				f.args = append(f.args, args...)
				return nil
			}
			f.args = append(f.args, s)
			continue
		}

		if s[1] == '-' {
			if len(s) == 2 { // "--" terminates the flags
				f.argsLenAtDash = len(f.args)
				f.args = append(f.args, args...)
				break
			}
			args, err = f.parseLongArg(s, args, fn)
		} else {
			args, err = f.parseShortArg(s, args, fn)
		}
		if err != nil {
			if errUnknownFlag, ok := err.(*unknownFlagError); ok { //nolint:errorlint
				// this means f.ParseErrorsAllowlist.UnknownFlagsHandling is set to UnknownFlagsHandlingPassUnknownToArgs
				f.args = append(f.args, errUnknownFlag.UnknownFlags)
				err = nil
			} else {
				return
			}
		}
	}
	return
}

// Parse parses flag definitions from the argument list, which should not
// include the command name.  Must be called after all flags in the FlagSet
// are defined and before flags are accessed by the program.
// The return value will be ErrHelp if -help was set but not defined.
func (f *FlagSet) Parse(arguments []string) error {
	if f.addedGoFlagSets != nil {
		for _, goFlagSet := range f.addedGoFlagSets {
			_ = goFlagSet.Parse(nil)
		}
	}
	f.parsed = true

	f.args = make([]string, 0, len(arguments))

	set := func(_ *innerflag, name, value string) error {
		return f.SetText(name, value)
	}

	var err error
	if len(arguments) > 0 {
		err = f.parseArgs(arguments, set)
	}
	if err == nil {
		err = f.applyEnv(nil)
	}
	if err != nil {
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			if errors.Is(err, ErrHelp) {
				os.Exit(0)
			}
			_, _ = fmt.Fprintln(f.Output(), err)
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
	}
	return nil
}

type parseFunc func(flag *innerflag, name, value string) error

// ParseAll parses flag definitions from the argument list, which should not
// include the command name. The arguments for fn are flag and value. The
// callback handles command-line values only; it may collect them without
// setting the flags. Environment values are then applied to flags not seen
// on the command line and not already changed. Must be called after all flags
// in the FlagSet are defined and before flags are accessed by the program.
// The return value will be ErrHelp if -help was set but not defined.
func (f *FlagSet) ParseAll(arguments []string, fn func(flag FlagMeta, value string) error) error {
	f.parsed = true
	f.args = make([]string, 0, len(arguments))

	seen := make(map[*innerflag]bool)
	set := func(flag *innerflag, _, value string) error {
		if err := fn(copyFlagMeta(flag.FlagMeta), value); err != nil {
			return err
		}
		seen[flag] = true
		return nil
	}

	var err error
	if len(arguments) > 0 {
		err = f.parseArgs(arguments, set)
	}
	if err == nil {
		err = f.applyEnv(seen)
	}
	if err != nil {
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			if errors.Is(err, ErrHelp) {
				os.Exit(0)
			}
			_, _ = fmt.Fprintln(f.Output(), err)
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
	}
	return nil
}

// Parsed reports whether f.Parse has been called.
func (f *FlagSet) Parsed() bool {
	return f.parsed
}

// Parse parses the command-line flags from os.Args[1:].  Must be called
// after all flags are defined and before flags are accessed by the program.
func Parse() {
	// Ignore errors; CommandLine is set for ExitOnError.
	_ = CommandLine.Parse(os.Args[1:])
}

// ParseAll parses the command-line flags from os.Args[1:] and called fn for each.
// The arguments for fn are flag and value. Must be called after all flags are
// defined and before flags are accessed by the program.
func ParseAll(fn func(flag FlagMeta, value string) error) {
	// Ignore errors; CommandLine is set for ExitOnError.
	_ = CommandLine.ParseAll(os.Args[1:], fn)
}

// SetInterspersed sets whether to support interspersed option/non-option arguments.
func SetInterspersed(interspersed bool) {
	CommandLine.SetInterspersed(interspersed)
}

// Parsed returns true if the command-line flags have been parsed.
func Parsed() bool {
	return CommandLine.Parsed()
}

// CommandLine is the default set of command-line flags, parsed from os.Args.
var CommandLine = NewFlagSet(os.Args[0], ExitOnError)

// NewFlagSet returns a new, empty flag set with the specified name,
// error handling property and SortFlags set to true.
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := &FlagSet{
		name:          name,
		errorHandling: errorHandling,
		argsLenAtDash: -1,
		interspersed:  true,
		SortFlags:     true,
	}
	return f
}

// SetInterspersed sets whether to support interspersed option/non-option arguments.
func (f *FlagSet) SetInterspersed(interspersed bool) {
	f.interspersed = interspersed
}

// Init sets the name and error handling property for a flag set.
// By default, the zero FlagSet uses an empty name and the
// ContinueOnError error handling policy.
func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.name = name
	f.errorHandling = errorHandling
	f.argsLenAtDash = -1
}
