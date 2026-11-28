// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gpflag implements POSIX/GNU-style command-line flags with generic
// construction and typed retrieval. gpflag is a hard fork of spf13/pflag
// intended to support modern Go syntax and features.
//
// Flags belong to a FlagSet; there is no package-level default set. The generic
// FlagSet methods New and NewP register a flag and return a pointer to its
// value. NewVar and NewVarP bind a variable the caller owns and return the
// non-generic *Flag, so that it can be configured:
//
//	fs := gpflag.NewFlagSet("server", gpflag.ContinueOnError)
//	port := fs.NewP("port", "p", uint16(8080), "listen port")
//	var debug bool
//	fs.NewVar(&debug, "debug", false, "enable debugging")
//	if err := fs.Parse(args); err != nil {
//		return err
//	}
//	listen(*port, debug)
//
// Native scalars, named primitive types, slices, and string-keyed maps use shared
// parsers. Standard-library types such as time.Duration, time.Time, net.IP,
// net.IPMask, and net.IPNet keep their Go types and usual text syntax. Types
// implementing encoding.TextUnmarshaler are recognized automatically. Custom Value
// implementations take precedence; package pkg/types provides Count, StringArray,
// Hex, Base64, BoolFunc, and a multi-layout Time value.
//
// A flag can also be read by name, or from its *Flag, with Get[T]. Get performs
// a type assertion on the actual stored value and panics if T is not the
// flag's type: TryGet[T] returns an error instead.
//
// Slices and maps retain normal Go sharing semantics. Slice/map defaults are copied
// at construction; the first occurrence replaces defaults and later occurrences
// append or merge. Slice values accept CSV: StringArray keeps each occurrence as
// one string.
//
// Long options accept --name=value or --name value. Booleans can omit the value.
// One-letter shorthands accept -p8080, -p=8080, or -p 8080. Boolean shorthands can
// be combined, as in -abc for flags -a -b -c. Flags and arguments can be
// interspersed until "--" is encountered. SetInterspersed(false) stops parsing
// at the first argument. Args, Arg, NArg and ArgsLenAtDash expose remaining arguments.
//
// Independent FlagSets support subcommands, normalization,
// hidden/deprecated flags, annotations, and configurable error and unknown-flag handling.
package gpflag

import (
	"bytes"
	"errors"
	goflag "flag"
	"fmt"
	"io"
	"os"
	"slices"
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
	// UnknownFlagsHandling decides how to handle unknown flags. Defaults to ErrorOnUnknownFlag.
	UnknownFlagsHandling UnknownFlagsHandling
}

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

	name              string
	parsed            bool
	actual            map[NormalizedName]*Flag
	orderedActual     []*Flag
	sortedActual      []*Flag
	formal            map[NormalizedName]*Flag
	orderedFormal     []*Flag
	sortedFormal      []*Flag
	shorthands        map[byte]*Flag
	args              []string // arguments after flags
	argsLenAtDash     int      // len(args) when a '--' was located when parsing, or -1 if no --
	errorHandling     ErrorHandling
	output            io.Writer // nil means stderr; use Output() accessor
	interspersed      bool      // allow interspersed option/non-option args
	normalizeNameFunc func(f *FlagSet, name string) NormalizedName

	addedGoFlagSets []*goflag.FlagSet
}

// A Flag represents the state of a flag.
type Flag struct {
	Name                string              // name as it appears on command line
	Shorthand           string              // one-letter abbreviated flag
	Usage               string              // help message
	Value               Value               // value as set
	DefValue            string              // default value (as text); for usage message
	Changed             bool                // If the user set the value (or if left to default)
	NoOptDefVal         string              // default value (as text); if the flag is on the command line without any options
	Deprecated          string              // If this flag is deprecated, this string is the new or now thing to use
	Hidden              bool                // used by cobra.Command to allow flags to be hidden from help/usage text
	ShorthandDeprecated string              // If the shorthand of this flag is deprecated, this string is the new or now thing to use
	Annotations         map[string][]string // used by cobra.Command bash autocomple code

	usageSyntax *string // set by SetUsageSyntax. Nil falls back to the default text
}

// SetUsageSyntax sets the text that help output prints directly after the
// flag's name, in place of the default such as " string" or "[=true|false]".
// The text is printed as given, so it must include any separating space.
// An empty string prints the name alone.
func (f *Flag) SetUsageSyntax(syntax string) {
	f.usageSyntax = &syntax
}

// sortFlags returns the flags as a slice in lexicographical sorted order.
func sortFlags(flags map[NormalizedName]*Flag) []*Flag {
	list := make([]string, len(flags))
	i := 0
	for k := range flags {
		list[i] = string(k)
		i++
	}
	slices.Sort(list)
	result := make([]*Flag, len(list))
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
	f.normalizeNameFunc = n
	f.sortedFormal = f.sortedFormal[:0]
	for fname, flag := range f.formal {
		nname := f.normalizeFlagName(flag.Name)
		if fname == nname {
			continue
		}
		flag.Name = string(nname)
		delete(f.formal, fname)
		f.formal[nname] = flag
		if _, set := f.actual[fname]; set {
			delete(f.actual, fname)
			f.actual[nname] = flag
		}
	}
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

// VisitAll visits the flags in lexicographical order or
// in primordial order if f.SortFlags is false, calling fn for each.
// It visits all flags, even those not set.
func (f *FlagSet) VisitAll(fn func(*Flag)) {
	if len(f.formal) == 0 {
		return
	}

	var flags []*Flag
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

// Visit visits the flags in lexicographical order or
// in primordial order if f.SortFlags is false, calling fn for each.
// It visits only those flags that have been set.
func (f *FlagSet) Visit(fn func(*Flag)) {
	if len(f.actual) == 0 {
		return
	}

	var flags []*Flag
	if f.SortFlags {
		if len(f.actual) != len(f.sortedActual) {
			f.sortedActual = sortFlags(f.actual)
		}
		flags = f.sortedActual
	} else {
		flags = f.orderedActual
	}

	for _, flag := range flags {
		fn(flag)
	}
}

// Lookup returns the Flag structure of the named flag, returning nil if none exists.
func (f *FlagSet) Lookup(name string) *Flag {
	return f.lookup(f.normalizeFlagName(name))
}

// ShorthandLookup returns the Flag structure of the short handed flag,
// returning nil if none exists.
// It panics, if len(name) > 1.
func (f *FlagSet) ShorthandLookup(name string) *Flag {
	if name == "" {
		return nil
	}
	if len(name) > 1 {
		msg := fmt.Sprintf("can not look up shorthand which is more than one ASCII character: %q", name)
		_, _ = fmt.Fprint(f.Output(), msg)
		panic(msg)
	}
	c := name[0]
	return f.shorthands[c]
}

// lookup returns the Flag structure of the named flag, returning nil if none exists.
func (f *FlagSet) lookup(name NormalizedName) *Flag {
	return f.formal[name]
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
	flag := f.Lookup(name)
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
	flag := f.Lookup(name)
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
	flag := f.Lookup(name)
	if flag == nil {
		return &NotExistError{name: name, messageType: flagNotExistMessage}
	}
	flag.Hidden = true
	return nil
}

// Set sets the value of the named flag.
func (f *FlagSet) Set(name, value string) error {
	normalName := f.normalizeFlagName(name)
	flag, ok := f.formal[normalName]
	if !ok {
		return &NotExistError{name: name, messageType: flagNoSuchFlagMessage}
	}

	var err error
	if setter, ok := flag.Value.(FirstSetter); ok {
		err = setter.SetFirst(value, !flag.Changed)
	} else {
		err = flag.Value.Set(value)
	}
	if err != nil {
		return &InvalidValueError{
			flag:  flag,
			value: value,
			cause: err,
		}
	}

	if !flag.Changed {
		if f.actual == nil {
			f.actual = make(map[NormalizedName]*Flag)
		}
		f.actual[normalName] = flag
		f.orderedActual = append(f.orderedActual, flag)

		flag.Changed = true
	}

	if flag.Deprecated != "" {
		_, _ = fmt.Fprintf(f.Output(), "Flag --%s has been deprecated, %s\n", name, flag.Deprecated)
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
	flag.Annotations[key] = values
	return nil
}

// Changed returns true if the flag was explicitly set during Parse() and false
// otherwise
func (f *FlagSet) Changed(name string) bool {
	flag := f.Lookup(name)
	// If a flag doesn't exist, it wasn't changed....
	if flag == nil {
		return false
	}
	return flag.Changed
}

// PrintDefaults prints, to standard error unless configured
// otherwise, the default values of all defined flags in the set.
func (f *FlagSet) PrintDefaults() {
	usages := f.FlagUsages()
	_, _ = fmt.Fprint(f.Output(), usages)
}

// defaultIsZeroValue returns true if the default value for this flag represents
// a zero value.
func (f *Flag) defaultIsZeroValue() bool {
	if z, ok := f.Value.(zeroDefaulter); ok {
		return f.DefValue == z.zeroDefault()
	}
	if isNoOptBoolValue(f.Value) {
		return f.DefValue == "false" || f.DefValue == ""
	}
	switch f.DefValue {
	case "", "0", "false", "<nil>", "[]", "0s":
		return true
	default:
		return false
	}
}

// UnquoteUsage extracts a back-quoted name from the usage
// string for a flag and returns it and the un-quoted usage.
// Given "a `name` to show" it returns ("name", "a name to show").
// If there are no back quotes, the name is an educated guess of the
// type of the flag's value, or the empty string if the flag is boolean.
func UnquoteUsage(flag *Flag) (name string, usage string) {
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

	name = flag.Value.Type()
	if isNoOptBoolValue(flag.Value) {
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

// defaultUsageSyntax returns the default text printed after a flag's name:
// the default value placeholder, then the value used when the flag has no argument.
func defaultUsageSyntax(flag *Flag, varname string) string {
	syntax := ""
	valueType := flag.Value.Type()
	if isNoOptBoolValue(flag.Value) && valueType == "bool" {
		syntax = "[=true|false]"
	} else if varname != "" {
		syntax = " " + varname
	}

	if flag.NoOptDefVal != "" {
		switch valueType {
		case "string":
			syntax += fmt.Sprintf("[=\"%s\"]", flag.NoOptDefVal)
		case "bool", "boolfunc":
			if flag.NoOptDefVal != "true" {
				syntax += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
			}
		case "count":
			if flag.NoOptDefVal != "+1" {
				syntax += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
			}
		default:
			syntax += fmt.Sprintf("[=%s]", flag.NoOptDefVal)
		}
	}
	return syntax
}

// FlagUsagesWrapped returns a string containing the usage information
// for all flags in the FlagSet. Wrapped to `cols` columns (0 for no
// wrapping)
func (f *FlagSet) FlagUsagesWrapped(cols int) string {
	buf := new(bytes.Buffer)

	lines := make([]string, 0, len(f.formal))

	maxlen := 0
	f.VisitAll(func(flag *Flag) {
		if flag.Hidden {
			return
		}

		line := ""
		if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
			line = fmt.Sprintf("  -%s, --%s", flag.Shorthand, flag.Name)
		} else {
			line = fmt.Sprintf("      --%s", flag.Name)
		}

		varname, usage := UnquoteUsage(flag)
		valueType := flag.Value.Type()

		var syntax string
		if flag.usageSyntax != nil {
			syntax = *flag.usageSyntax
		} else {
			syntax = defaultUsageSyntax(flag, varname)
		}
		line += syntax

		// This special character will be replaced with spacing once the
		// correct alignment is calculated
		line += "\x00"
		if len(line) > maxlen {
			maxlen = len(line)
		}

		line += usage
		if !flag.defaultIsZeroValue() {
			if valueType == "string" {
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

// defaultUsage is the default function to print a usage message.
func defaultUsage(f *FlagSet) {
	_, _ = fmt.Fprintf(f.Output(), "Usage of %s:\n", f.name)
	f.PrintDefaults()
}

// NFlag returns the number of flags that have been set.
func (f *FlagSet) NFlag() int { return len(f.actual) }

// Arg returns the i'th argument.  Arg(0) is the first remaining argument
// after flags have been processed.
func (f *FlagSet) Arg(i int) string {
	if i < 0 || i >= len(f.args) {
		return ""
	}
	return f.args[i]
}

// NArg is the number of arguments remaining after flags have been processed.
func (f *FlagSet) NArg() int { return len(f.args) }

// Args returns the non-flag arguments.
func (f *FlagSet) Args() []string { return f.args }

// AddFlag will add the flag to the FlagSet
func (f *FlagSet) AddFlag(flag *Flag) {
	normalizedFlagName := f.normalizeFlagName(flag.Name)

	_, alreadyThere := f.formal[normalizedFlagName]
	if alreadyThere {
		msg := fmt.Sprintf("%s flag redefined: %s", f.name, flag.Name)
		_, _ = fmt.Fprintln(f.Output(), msg)
		panic(msg) // Happens only if flags are declared with identical names
	}
	if f.formal == nil {
		f.formal = make(map[NormalizedName]*Flag)
	}

	flag.Name = string(normalizedFlagName)
	if flag.NoOptDefVal == "" {
		if d, ok := flag.Value.(NoOptDefaulter); ok {
			flag.NoOptDefVal = d.NoOptDefValue()
		} else if isNoOptBoolValue(flag.Value) {
			flag.NoOptDefVal = "true"
		}
	}
	f.formal[normalizedFlagName] = flag
	f.orderedFormal = append(f.orderedFormal, flag)

	if flag.Shorthand == "" {
		return
	}
	if len(flag.Shorthand) > 1 {
		msg := fmt.Sprintf("%q shorthand is more than one ASCII character", flag.Shorthand)
		_, _ = fmt.Fprint(f.Output(), msg)
		panic(msg)
	}
	if f.shorthands == nil {
		f.shorthands = make(map[byte]*Flag)
	}
	c := flag.Shorthand[0]
	used, alreadyThere := f.shorthands[c]
	if alreadyThere {
		msg := fmt.Sprintf("unable to redefine %q shorthand in %q flagset: it's already used for %q flag", c, f.name, used.Name)
		_, _ = fmt.Fprint(f.Output(), msg)
		panic(msg)
	}
	f.shorthands[c] = flag
}

// AddFlagSet adds one FlagSet to another. If a flag is already present in f
// the flag from newSet will be ignored.
func (f *FlagSet) AddFlagSet(newSet *FlagSet) {
	if newSet == nil {
		return
	}
	newSet.VisitAll(func(flag *Flag) {
		if f.Lookup(flag.Name) == nil {
			f.AddFlag(flag)
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

// usage calls the Usage method for the flag set, or the default usage
// function when none is set.
func (f *FlagSet) usage() {
	if f.Usage == nil {
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

	name, value, hasValue := strings.Cut(name, "=")
	flag, exists := f.formal[f.normalizeFlagName(name)]
	unknownFlagsHandling := f.ParseErrorsAllowlist.UnknownFlagsHandling

	if !exists {
		switch {
		case name == "help":
			f.usage()
			return a, ErrHelp
		case unknownFlagsHandling == IgnoreUnknownFlag:
			// --unknown=unknownval arg ...
			// we do not want to lose arg in this case
			if hasValue {
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

	switch {
	case hasValue:
		// '--flag=arg': value was provided after '='.
	case flag.NoOptDefVal != "":
		// '--flag' (arg was optional)
		value = flag.NoOptDefVal
	case len(a) > 0:
		// '--flag arg'
		value = a[0]
		a = a[1:]
	default:
		// '--flag' (arg was required)
		err = f.fail(&ValueRequiredError{
			flag:          flag,
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
		unknownFlagsHandling := f.ParseErrorsAllowlist.UnknownFlagsHandling

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
			flag:                flag,
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
				// this means f.ParseErrorsAllowlist.UnknownFlagsHandling is PassUnknownFlagToArgs
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
				// this means f.ParseErrorsAllowlist.UnknownFlagsHandling is PassUnknownFlagToArgs
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

	if len(arguments) == 0 {
		return nil
	}

	set := func(_ *Flag, name, value string) error {
		return f.Set(name, value)
	}

	err := f.parseArgs(arguments, set)
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

type parseFunc func(flag *Flag, name, value string) error

// ParseAll parses flag definitions from the argument list, which should not
// include the command name. The arguments for fn are flag and value. Must be
// called after all flags in the FlagSet are defined and before flags are
// accessed by the program. The return value will be ErrHelp if -help was set
// but not defined.
func (f *FlagSet) ParseAll(arguments []string, fn func(flag *Flag, value string) error) error {
	f.parsed = true
	f.args = make([]string, 0, len(arguments))

	set := func(flag *Flag, _, value string) error {
		return fn(flag, value)
	}

	err := f.parseArgs(arguments, set)
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
