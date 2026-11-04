package gpflag

import (
	"fmt"
	"maps"
	"os"
	"reflect"
)

// FlagMeta describes a flag, without exposing its parser or value. Public
// snapshots, including their annotations, can be mutated without changing it.
type FlagMeta struct {
	Name, Shorthand, Usage          string
	Type, DefValue                  string
	Changed                         bool
	NoOptDefVal                     string
	Deprecated, ShorthandDeprecated string
	Hidden                          bool
	Annotations                     map[string][]string
	zeroDefault                     bool
	boolFlag                        bool
	env                             string
}

func copyFlagMeta(meta FlagMeta) FlagMeta {
	meta.Annotations = maps.Clone(meta.Annotations)
	for key, values := range meta.Annotations {
		meta.Annotations[key] = append([]string(nil), values...)
	}
	return meta
}

// flagOps erases behavior only. Metadata remains directly accessible on the
// concrete innerflag; the implementation is the Flag[T] itself or a Go import.
type flagOps interface {
	setText(string) error
	text() string
	cloneFor(*FlagSet) *innerflag
}

// innerflag is the set's non-generic view into a flag. It lives inline in its
// owning Flag[T] (or importedFlag), not in a separately allocated wrapper.
type innerflag struct {
	FlagMeta
	flagOps
	fs     *FlagSet
	native bool // parsing can use a native standard-library registration
}

// Option configures parsing behavior for a particular Go value type.
// Options are created by Count, StringArray, Base64, TimeFormats, or WithParser.
type Option[T any] interface{ apply(*Flag[T]) }

// New registers a flag on CommandLine.
func New[T any](name string, value T, usage string, options ...Option[T]) *Flag[T] {
	return CommandLine.New(name, value, usage, options...)
}

// NewP is New with a single ASCII shorthand character.
func NewP[T any](name, shorthand string, value T, usage string, options ...Option[T]) *Flag[T] {
	return CommandLine.NewP(name, shorthand, value, usage, options...)
}

// NewVar registers a flag backed by p on CommandLine.
func NewVar[T any](p *T, name string, value T, usage string, options ...Option[T]) *Flag[T] {
	return CommandLine.NewVar(p, name, value, usage, options...)
}

// NewVarP is NewVar with a single ASCII shorthand character.
func NewVarP[T any](p *T, name, shorthand string, value T, usage string, options ...Option[T]) *Flag[T] {
	return CommandLine.NewVarP(p, name, shorthand, value, usage, options...)
}

// New registers a flag, allocating storage for its value. Unsupported types
// require WithParser or encoding.TextUnmarshaler support.
func (fs *FlagSet) New[T any](name string, value T, usage string, options ...Option[T]) *Flag[T] {
	return fs.NewVar(new(T), name, value, usage, options...)
}

// NewP is New with a single ASCII shorthand character.
func (fs *FlagSet) NewP[T any](name, shorthand string, value T, usage string, options ...Option[T]) *Flag[T] {
	return fs.NewVarP(new(T), name, shorthand, value, usage, options...)
}

// NewVar registers a flag backed by p. Direct writes to p are not tracked as
// assignments; use Flag.Set when change tracking is needed.
func (fs *FlagSet) NewVar[T any](p *T, name string, value T, usage string, options ...Option[T]) *Flag[T] {
	return fs.NewVarP(p, name, "", value, usage, options...)
}

// NewVarP is NewVar with a single ASCII shorthand character.
func (fs *FlagSet) NewVarP[T any](p *T, name, shorthand string, value T, usage string, options ...Option[T]) *Flag[T] {
	if p == nil {
		panic("gpflag: nil bound variable")
	}
	f := &Flag[T]{
		inner: innerflag{fs: fs, FlagMeta: FlagMeta{Name: name, Usage: usage, Type: reflect.TypeFor[T]().String()}},
		value: p,
	}
	configureParser(f)
	for _, option := range options {
		if option == nil {
			panic("gpflag: empty option")
		}
		option.apply(f)
	}
	if f.codec == nil {
		panic(fmt.Sprintf("gpflag: cannot define a flag of type %s: use WithParser or encoding.TextUnmarshaler", reflect.TypeFor[T]()))
	}
	if isNilPointer(value) {
		panic(fmt.Sprintf("gpflag: cannot define a flag with a nil %s value", reflect.TypeFor[T]()))
	}
	f.inner.DefValue = f.codec.format(value)
	f.inner.zeroDefault = zeroDefault(value)
	f.inner.Shorthand = shorthand
	f.inner.flagOps = f
	fs.addFlag(&f.inner)
	*p = value
	return f
}

// Lookup retrieves the actual registered flag. T must exactly match its Go
// type. Missing names, mismatches, and opaque imported flags return an error.
func (fs *FlagSet) Lookup[T any](name string) (*Flag[T], error) {
	entry := fs.formal[fs.normalizeFlagName(name)]
	if entry == nil {
		return nil, &NotExistError{name: name, messageType: flagNotExistMessage}
	}
	if _, opaque := entry.flagOps.(*importedFlag); opaque {
		return nil, fmt.Errorf("flag %q is an opaque imported Go flag, not *Flag[%s]", name, reflect.TypeFor[T]())
	}
	f, ok := entry.flagOps.(*Flag[T])
	if !ok {
		return nil, fmt.Errorf("flag %q is %T, not *Flag[%s]", name, entry.flagOps, reflect.TypeFor[T]())
	}
	return f, nil
}

// Lookup retrieves a typed flag from CommandLine.
func Lookup[T any](name string) (*Flag[T], error) { return CommandLine.Lookup[T](name) }

func (f *Flag[T]) text() string { return f.codec.format(*f.value) }
func (f *Flag[T]) cloneFor(fs *FlagSet) *innerflag {
	clone := *f
	clone.inner.FlagMeta = copyFlagMeta(f.inner.FlagMeta)
	clone.inner.fs, clone.inner.Changed = fs, false
	clone.inner.flagOps = &clone
	return &clone.inner
}
func (f *Flag[T]) setText(text string) error {
	next, err := f.codec.parse(*f.value, text, !f.inner.Changed)
	if err != nil {
		return err
	}
	*f.value = next
	return nil
}

// Meta returns an isolated snapshot of the flag's metadata.
func (f *Flag[T]) Meta() FlagMeta { return copyFlagMeta(f.inner.FlagMeta) }

// Get returns the current Go value. Reference-valued data is not copied.
func (f *Flag[T]) Get() T { return *f.value }

// Ptr returns the bound variable. Direct writes bypass change tracking.
func (f *Flag[T]) Ptr() *T { return f.value }

// Set assigns a Go value, replacing any existing collection, and records it as
// changed. It does not interpret command-line text or merge collections.
func (f *Flag[T]) Set(value T) {
	*f.value = value
	f.inner.fs.recordChanged(&f.inner)
}

// SetText parses text according to the flag's configured syntax.
func (f *Flag[T]) SetText(text string) error { return f.inner.fs.SetText(f.inner.Name, text) }

// String formats the current value.
func (f *Flag[T]) String() string { return f.text() }

// Short changes the shorthand, checking for conflicts before modifying it.
func (f *Flag[T]) Short(r rune) *Flag[T] {
	shorthand := ""
	if r != 0 {
		shorthand = string(r)
	}
	f.inner.fs.setShorthand(&f.inner, shorthand)
	return f
}

// Env selects environment fallback. An empty name removes it.
func (f *Flag[T]) Env(name string) *Flag[T] { f.inner.env = name; return f }
func (f *Flag[T]) EnvName() string          { return f.inner.env }

func (f *Flag[T]) MarkDeprecated(message string) *Flag[T] {
	f.inner.Deprecated, f.inner.Hidden = message, true
	return f
}
func (f *Flag[T]) MarkShorthandDeprecated(message string) *Flag[T] {
	f.inner.ShorthandDeprecated = message
	return f
}
func (f *Flag[T]) MarkHidden() *Flag[T] { f.inner.Hidden = true; return f }
func (f *Flag[T]) SetNoOptDefVal(value string) *Flag[T] {
	f.inner.NoOptDefVal = value
	return f
}
func (f *Flag[T]) SetAnnotations(annotations map[string][]string) *Flag[T] {
	f.inner.Annotations = copyFlagMeta(FlagMeta{Annotations: annotations}).Annotations
	return f
}

func (fs *FlagSet) recordChanged(flag *innerflag) {
	if flag.Changed {
		return
	}
	if fs.actual == nil {
		fs.actual = make(map[NormalizedName]*innerflag)
	}
	fs.actual[NormalizedName(flag.Name)] = flag
	fs.orderedActual = append(fs.orderedActual, flag)
	flag.Changed = true
}

func (fs *FlagSet) applyEnv(seen map[*innerflag]bool) error {
	var result error
	fs.visitAll(func(flag *innerflag) {
		if result != nil || flag.Changed || seen[flag] || flag.env == "" {
			return
		}
		text, exists := os.LookupEnv(flag.env)
		if !exists {
			return
		}
		if err := flag.setText(text); err != nil {
			result = fmt.Errorf("invalid value %q for environment variable %s (flag %q): %w", text, flag.env, flag.Name, err)
			return
		}
		fs.recordChanged(flag)
	})
	return result
}

func isNilPointer[T any](value T) bool {
	v := reflect.ValueOf(value)
	return v.IsValid() && v.Kind() == reflect.Pointer && v.IsNil()
}

func zeroDefault[T any](value T) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	default:
		return v.IsZero()
	}
}
