# gpflag — Generic POSIX Flags

`gpflag` is a generics-native hard fork of
[`spf13/pflag`](https://github.com/spf13/pflag), requiring **Go 1.27+**.
It retains GNU/POSIX command-line parsing, but not pflag's Go API compatibility.

```sh
go get github.com/naja-go/gpflag@latest
```

## Quick start

```go
package main

import (
    "fmt"
    "github.com/naja-go/gpflag"
)

func main() {
    port := gpflag.NewP("port", "p", 8080, "listen port").Env("APP_PORT")
    tags := gpflag.New[[]string]("tag", nil, "tags")
    verbose := gpflag.NewP("verbose", "v", false, "verbose output")

    gpflag.Parse()
    fmt.Println(port.Get(), tags.Get(), verbose.Get())
}
```

`New[T]` returns the registered `*Flag[T]`, with one bound variable and an inline
private `innerflag`. The set indexes that concrete view: metadata is directly
accessible, while a small internal interface dispatches behavior to the owning
flag. Typed codecs hold parsing configuration, not values or occurrence state.
There is no `Value` interface or stored dispatch closure. Flags must not be copied.

## Constructors and flag sets

The same constructors exist on `gpflag.CommandLine` and individual sets:

```go
fs := gpflag.NewFlagSet("serve", gpflag.ContinueOnError)
port := fs.NewP("port", "p", 8080, "listen port")

var host string
fs.NewVar(&host, "host", "localhost", "listen host")
var secure bool
fs.NewVarP(&secure, "secure", "s", false, "enable TLS")

if err := fs.Parse(os.Args[1:]); err != nil {
    return err
}
fmt.Println(port.Get(), host, secure)
```

- `New(name, default, usage)` allocates the bound variable.
- `NewP(name, shorthand, default, usage)` also registers a shorthand.
- `NewVar(pointer, name, default, usage)` binds an existing variable.
- `NewVarP(pointer, name, shorthand, default, usage)` does both.

All accept trailing typed options. Shorthands are single ASCII characters;
`.Short('p')` can also set or replace one after registration. Invalid definitions
(duplicate names/shorthands, nil binding pointers, unsupported types) panic
before publishing the registration.

## Values and text

```go
port.Set(9090)                    // typed assignment; records the change
err := port.SetText("9090")       // parse command-line text
err = fs.SetText("port", "9090") // parse text by name
```

`Set(T)` replaces the current value, including collections. Text assignments
follow the flag's repetition rules: slices append after the first assignment,
and maps merge. The first text assignment replaces collection defaults.

`Get()` returns `T` without deep copying. Slices, maps, and pointers share their
underlying storage. `Ptr()` exposes the bound variable. Direct writes through
these references cannot participate in assignment tracking; use `Set` or
`SetText` when tracking is needed.

## Supported types and parsing options

Built-in parsers support:

- `string`, `bool`, all signed/unsigned integer widths, `float32`, `float64`;
- `time.Duration`, `time.Time`;
- `net.IP`, `net.IPNet`, `net.IPMask`;
- `[]string`, `[]bool`, `[]int`, `[]int32`, `[]int64`, `[]uint`,
  `[]float32`, `[]float64`, `[]time.Duration`, `[]byte`, `[]net.IP`, `[]net.IPNet`;
- `map[string]string`, `map[string]int`, `map[string]int64`.

Parsing behavior is configuration, not a different Go value type:

```go
count := fs.NewP("verbose", "v", 0, "verbosity", gpflag.Count())
items := fs.New[[]string]("item", nil, "verbatim items", gpflag.StringArray())
blob := fs.New[[]byte]("blob", nil, "base64 data", gpflag.Base64())
date := fs.New("date", time.Time{}, "date", gpflag.TimeFormats("2006-01-02"))
```

`Count()` applies to `int`, `StringArray()` to `[]string`, `Base64()` to `[]byte`,
and `TimeFormats()` to `time.Time`. Inapplicable options are compile-time errors.
Options are sealed typed interfaces; a nil option is an invalid definition.
Plain `[]string` uses CSV syntax; plain `[]byte` uses hexadecimal. Time defaults
to RFC3339Nano. Integer widths are checked, including native `int`/`uint` widths.

## Looking up and inspecting flags

```go
port, err := fs.Lookup[int]("port")
if err != nil {
    return err
}
fmt.Println(port.Get(), port.Meta().Usage)
```

Lookup returns the registered object, not a new handle. It applies name
normalization and requires the exact Go type. For example, a count flag is an
`int`; a distinct named integer type is not interchangeable with it.
Missing names, mismatches, and opaque imported Go flags return a nil flag and
an error. `gpflag.Lookup[T]` searches `CommandLine`.

`Meta()` returns a `FlagMeta` snapshot with names, usage, presentation type,
original default text, change status, and annotations. Its annotation data is
copied. Changing a snapshot does not change the flag or its registration.
Use `VisitAll(func(gpflag.FlagMeta))` for all metadata and `Visit` for assigned flags.
`ShorthandLookup` also returns a snapshot (or nil).

## Environment fallback

```go
port := fs.New("port", 8080, "port").Env("APP_PORT")
```

Command-line or programmatic assignments take precedence over the environment,
which takes precedence over the default. Environment variables are read during
`Parse`/`ParseAll`; malformed values are errors. `.Env("")` removes the binding.

`ParseAll` sends command-line text and `FlagMeta` snapshots to a callback. The
callback can collect values without assigning them, or call `fs.SetText`.
Environment fallback happens afterward, without calling the callback, and never
replaces a flag successfully handled on the command line.

## Custom parsing

Types implementing `encoding.TextUnmarshaler` on `T` or `*T` can be registered
directly. `encoding.TextMarshaler`, when available, supplies formatting.
Non-nil custom pointers retain their identity. Nil custom pointer defaults are
rejected.

For other types, supply typed parsing and formatting explicitly. These callbacks
(and `Func` callbacks) are the intentional exception to method-based codecs:

```go
type level struct { n int }

lvl := fs.New("level", level{n: 1}, "log level", gpflag.WithParser(
    func(_ level, text string, first bool) (level, error) {
        n, err := strconv.Atoi(text)
        return level{n: n}, err
    },
    func(value level) string { return strconv.Itoa(value.n) },
))
```

The parser receives the current value and whether this is its first successful
assignment. Return the next value or an error. Do not mutate shared input data
before returning an error: custom pointer unmarshaling and callback side effects
cannot be rolled back. Built-in collection parsers validate incoming data before
mutating their destination.

There are no `Var`, `Value`, `SliceValue`, or special wrapper value types.
`Func`/`FuncP` and `BoolFunc`/`BoolFuncP` remain available for text callbacks.

## Inheriting flags

`dest.AddFlag(flag)` and `dest.AddFlagSet(source)` create independent
registrations sharing bound variables. Each set owns its metadata, normalized
names, environment binding, and assignment tracking. Subsequent lookups return
the destination's actual registration. `AddFlagSet` skips names already defined
in the destination. Sharing variables does not make concurrent parsing safe.

## Command-line syntax

Flags may be written as `--flag=value` or `--flag value`. Boolean flags and
count flags accept bare occurrences. Single-dash arguments are shorthand
clusters: `-vvv`, `-p8080`, or `-p 8080`. All but the final shorthand must accept
an omitted argument. `.SetNoOptDefVal("text")` configures omitted arguments for
other flag types.

Parsing stops at `--`. By default flags can be interspersed with positional
arguments; `SetInterspersed(false)` stops parsing at the first positional
argument. Integer flags accept decimal, octal, and hexadecimal notation;
booleans accept the forms supported by `strconv.ParseBool`.

## Configuration

```go
fs.SetNormalizeFunc(func(_ *gpflag.FlagSet, name string) gpflag.NormalizedName {
    return gpflag.NormalizedName(strings.ReplaceAll(name, "_", "-"))
})
fs.New("old", 0, "old flag").MarkDeprecated("use --new")
fs.New("secret", "", "secret flag").MarkHidden()
fs.SortFlags = false // preserve definition order in help and visits
```

Normalization collisions are rejected without changing registrations.
Metadata setters, including `.SetAnnotations` and `.MarkShorthandDeprecated`,
operate on the registered flag rather than a mutable public metadata struct.

## Standard-library interoperability

`AddGoFlagSet` imports standard-library flags as explicitly opaque entries.
Read them through their original bound variables; typed lookup does not guess
Go types from their parsers.

`CopyToGoFlagSet` shares variables with a standard-library set. Native scalar
registrations preserve Go's help formatting; custom parsers use edge adapters.
The exported set follows Go's argument syntax, not all POSIX omission rules.
Native writes bypass gpflag assignment tracking, just like direct variable
writes. `ParseSkippedFlags` handles the `-test.*` flags skipped by gpflag.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
```

GNU/POSIX syntax reference:
<https://sourceware.org/glibc/manual/latest/html_node/Argument-Syntax.html>

Licensed under the BSD license in [LICENSE](LICENSE).
