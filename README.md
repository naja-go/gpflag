# gpflag — generic POSIX flags

[![Build Status](https://github.com/naja-go/gpflag/actions/workflows/ci.yaml/badge.svg)](https://github.com/naja-go/gpflag/actions/workflows/ci.yaml)
![GitHub License](https://img.shields.io/github/license/naja-go/gpflag)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/naja-go/gpflag)](https://pkg.go.dev/github.com/naja-go/gpflag)

A **hard fork** of spf13/pflag, using Go generics instead of type-specific APIs.
Requires **Go 1.27+**, including generic methods. This is not source-compatible
with pflag or Go's `flag` package.

gpflag is forked from [spf13/pflag](https://github.com/spf13/pflag), which is
itself derived from Go's [`flag`](https://pkg.go.dev/flag) package. It is
distributed under the same BSD-style [license](LICENSE), which retains the
original copyright notices.

```sh
go get github.com/naja-go/gpflag
```

## Define, parse, retrieve

```go
package main

import (
    "fmt"
    "os"

    "github.com/naja-go/gpflag"
    "github.com/naja-go/gpflag/pkg/types"
)

func main() {
    fs := gpflag.NewFlagSet("server", gpflag.ContinueOnError)
    port := fs.NewP("port", "p", uint16(8080), "listen port")
    var verbose types.Count
    fs.NewVarP(&verbose, "verbose", "v", types.Count(0), "verbosity")
    if err := fs.Parse(os.Args[1:]); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(2)
    }
    fmt.Println(*port, int(verbose), fs.Args())
}
```

Flags are always created on a `FlagSet`:

```go
func (f *FlagSet) New[T any](name string, value T, usage string) *T
func (f *FlagSet) NewP[T any](name, shorthand string, value T, usage string) *T
func (f *FlagSet) NewVar[T any](p *T, name string, value T, usage string) *Flag
func (f *FlagSet) NewVarP[T any](p *T, name, shorthand string, value T, usage string) *Flag
```

`New` and `NewP` return a **typed pointer to the value**, as pflag's `Int` and
`String` did, so reading a flag is a dereference the compiler checks. `NewVar`
and `NewVarP` bind a variable you already hold and return the non-generic
**`*Flag`**, which is where you set `Hidden`, `Deprecated`, `NoOptDefVal`, or
read `Changed`. `fs.Lookup(name)` returns the `*Flag` for any flag.

There is **no global flag set**: unlike pflag and Go's `flag`, gpflag has no
`CommandLine` and no package-level `New`, `Parse`, `Lookup`, or `Args`. Create
one with `NewFlagSet` and pass `os.Args[1:]` to `Parse`.
`NewVar` initializes the supplied variable to the default. Unsupported types and
nil destination pointers panic at construction; invalid arguments return errors.

When the pointer is not at hand, for example when a flag is defined in one
place and read in another, retrieve the stored value by name or from its
`*Flag`, without formatting and reparsing:

```go
value := fs.Get[uint16]("port")
value := fs.Lookup("port").Get[uint16]()
value := gpflag.Get[uint16](fs.Lookup("port"))
```

`Get` panics if the flag does not exist or `T` is not its exact type, since both
are programming errors. `TryGet` has the same three forms and returns
`(T, error)` instead, for flags or types that are not known statically:

```go
value, err := fs.TryGet[uint16](name)
```

Retrieval uses a Go type assertion: `Get[int]` cannot retrieve a `uint16`, nor can
it discard a named type. Bound variables and returned slices/maps have ordinary
Go storage-sharing semantics. FlagSets are not safe for concurrent mutation.

## Supported values

- Booleans, strings, all signed/unsigned integers, `uintptr`, floats, complex
  numbers, and named versions of these types. Integer syntax follows
  `strconv.ParseInt`/`ParseUint` with base 0; destination-width overflow is rejected.
- Slices of scalar values, including named slice/element types. A `[]byte` is a
  numeric CSV list; use `types.Hex` or `types.Base64` for encoded binary data.
- String-keyed maps with scalar values, including named string keys.
- `time.Duration`, `time.Time` (RFC3339Nano), `net.IP`, `net.IPMask`, and `net.IPNet`.
  These also work as collection elements. Empty scalar IP input retains its
  existing value, as in pflag.
- Types implementing `encoding.TextUnmarshaler`, including standard-library
  values such as `netip.Addr`. `encoding.TextMarshaler` supplies formatting when
  available; otherwise formatting uses `fmt.Sprint`.
- `func(string) error` callbacks, called for each occurrence.
- Custom values: a type whose pointer implements `gpflag.Value`, or a non-nil
  pointer that is itself a `Value`. Bind one you already hold with
  `fs.NewVar(&level, "level", level, "usage")`. The
  flag's `Value` is the caller's own pointer, never a wrapper.

`Value.Type()` and help output name native flags by kind (`uint16`,
`uint16Slice`, `stringToUint16`), including for named types such as
`type Port uint16`. `TextUnmarshaler` types are named `text`. The exact Go type is still what `Get[Port]` requires.

Native collections accept CSV. The **first successful occurrence replaces the
default**, and later occurrences append to slices or merge maps:

```go
fs.New("tags", []string{"default"}, "tags")
// --tags=a,b --tags=c -> []string{"a", "b", "c"}
fs.New("limits", map[string]int{"default": 1}, "limits")
// --limits=a=2,b=3 --limits=a=4 -> map[string]int{"a": 4, "b": 3}
```

Slice/map defaults are copied, so parsing does not modify the supplied default.
Nil and empty defaults remain distinct until explicitly set. An empty CSV value
creates an empty collection. Invalid native parsing does not modify storage or
advance first-occurrence state. Slice flags expose `SliceValue` for append,
replace, and string-list access. Maps keep stable working storage across parses;
formatting sorts keys for deterministic help output.

String maps retain pflag's single-assignment rule: `--define=a=1,2` assigns
`"1,2"` to `a`. Multiple assignments use CSV; quote entire `key=value` fields to
preserve commas, equals signs, or newlines. Multiple unquoted CSV records in one
argument are rejected.

## Special types

`github.com/naja-go/gpflag/pkg/types` provides:

| Type | Behavior |
| --- | --- |
| `Count` | Increments without an argument; `-vvv` produces 3. Explicit values set the count. |
| `StringArray` | Each occurrence is one string, without CSV splitting. `--item=` retains one empty string. |
| `Hex` | Hexadecimal bytes, displayed uppercase. |
| `Base64` | Standard padded base64 bytes. |
| `BoolFunc` | Callback accepting boolean no-argument syntax. |
| `Time` | Embedded `time.Time` with a `Formats []string` list of accepted layouts. |

Custom `Value` implementations take precedence over native parsing. A `Value`
may also implement these optional interfaces:

| Interface | Method | Effect |
| --- | --- | --- |
| `BoolFlag` | `IsBoolFlag() bool` | The flag may be given without an argument, meaning `true`. |
| `NoOptDefaulter` | `NoOptDefValue() string` | Supplies the text parsed when the flag has no argument. |
| `FirstSetter` | `SetFirst(text string, first bool) error` | Called instead of `Set`; `first` is true until the flag is `Changed`. |
| `Getter` | `Get() any` | Supplies the value for `Get[T]` when the `Value` is not a `*T`. |

The hooks apply however the flag is registered: `New`, `NewVar`, or `AddFlag`
with a hand-built `Flag`. They are applied by the `FlagSet`, so calling
`Value.Set` directly bypasses `SetFirst`. That includes the standard library:
after `CopyToGoFlagSet`, a `StringArray` parsed by Go's `flag` appends to its
default. Custom setters are responsible for
leaving their own storage unchanged on error.

## POSIX/GNU behavior

- Long options: `--port=8080` or `--port 8080`.
- Shorthands: `-p8080`, `-p=8080`, or `-p 8080`; combined booleans: `-abc`.
- Boolean no-argument syntax, explicit `--debug=false`, and `NoOptDefVal`.
- Interspersed arguments, `--` termination, `ArgsLenAtDash`, and optional
  `SetInterspersed(false)`.
- Name normalization, sorted/registration-order visits, hidden/deprecated flags,
  shorthand deprecation, annotations, and wrapped help with backtick placeholders.
- Continue/exit/panic error handling and configurable unknown-flag handling.
- `AddGoFlagSet`, `CopyToGoFlagSet`, and `ParseSkippedFlags` for standard-library
  flag integration. Shared values are not reinitialized during copying.

## Development

```sh
go test -race ./...
go vet ./...
```

BSD licensed; see [LICENSE](LICENSE).
