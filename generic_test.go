package gpflag

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFlagInt(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "listen port").Short('p')

	if got := port.Get(); got != 8080 {
		t.Fatalf("default = %d, want 8080", got)
	}
	if port.Meta().Name != "port" {
		t.Fatalf("name = %q, want %q", port.Meta().Name, "port")
	}
	if port.Meta().Shorthand != "p" {
		t.Fatalf("shorthand = %q, want %q", port.Meta().Shorthand, "p")
	}
	if port.Meta().Type != "int" {
		t.Fatalf("type = %q, want int", port.Meta().Type)
	}
	if got, err := fs.Lookup[int]("port"); err != nil || got != port {
		t.Fatalf("Lookup = %v, %v; want original port handle", got, err)
	}
	if fs.formal["port"] != &port.inner || fs.shorthands['p'] != &port.inner || port.inner.flagOps != port {
		t.Fatal("the core was not registered with its typed owner")
	}

	if err := fs.Parse([]string{"-p", "9090"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := port.Get(); got != 9090 {
		t.Fatalf("value = %d, want 9090", got)
	}
	if !port.Meta().Changed {
		t.Fatalf("Changed = false, want true")
	}
}

func TestFlagVar(t *testing.T) {
	var n int
	fs := NewFlagSet("test", ContinueOnError)
	tf := fs.NewVar(&n, "n", 1, "number")

	if n != 1 {
		t.Fatalf("initial = %d, want 1", n)
	}
	if err := fs.Parse([]string{"--n=5"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n != 5 || tf.Get() != 5 {
		t.Fatalf("n = %d, Get = %d, want 5", n, tf.Get())
	}
	if tf.Ptr() != &n {
		t.Fatalf("Ptr did not return the bound variable")
	}
}

func TestFlagBuiltinTypes(t *testing.T) {
	ipnet := func(cidr string) net.IPNet {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("ParseCIDR(%q): %v", cidr, err)
		}
		return *n
	}

	testTyped(t, "string", "default", "value", "value", "string")
	testTyped(t, "bool", false, "true", true, "bool")
	testTyped(t, "int", 1, "42", 42, "int")
	testTyped(t, "int8", int8(1), "42", int8(42), "int8")
	testTyped(t, "int16", int16(1), "42", int16(42), "int16")
	testTyped(t, "int32", int32(1), "42", int32(42), "int32")
	testTyped(t, "int64", int64(1), "42", int64(42), "int64")
	testTyped(t, "uint", uint(1), "42", uint(42), "uint")
	testTyped(t, "uint8", uint8(1), "42", uint8(42), "uint8")
	testTyped(t, "uint16", uint16(1), "42", uint16(42), "uint16")
	testTyped(t, "uint32", uint32(1), "42", uint32(42), "uint32")
	testTyped(t, "uint64", uint64(1), "42", uint64(42), "uint64")
	testTyped(t, "float32", float32(1), "4.5", float32(4.5), "float32")
	testTyped(t, "float64", float64(1), "4.5", float64(4.5), "float64")
	testTyped(t, "duration", time.Second, "1h30m", 90*time.Minute, "duration")
	testTyped(t, "time", time.Time{}, "2024-01-02T03:04:05Z", time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), "time")
	testTyped(t, "stringSlice", []string(nil), "a,b", []string{"a", "b"}, "stringSlice")
	testTyped(t, "stringArray", []string(nil), "a,b", []string{"a,b"}, "stringArray", StringArray())
	testTyped(t, "boolSlice", []bool(nil), "true,false", []bool{true, false}, "boolSlice")
	testTyped(t, "intSlice", []int(nil), "1,2", []int{1, 2}, "intSlice")
	testTyped(t, "int32Slice", []int32(nil), "1,2", []int32{1, 2}, "int32Slice")
	testTyped(t, "int64Slice", []int64(nil), "1,2", []int64{1, 2}, "int64Slice")
	testTyped(t, "uintSlice", []uint(nil), "1,2", []uint{1, 2}, "uintSlice")
	testTyped(t, "float32Slice", []float32(nil), "1.5,2.5", []float32{1.5, 2.5}, "float32Slice")
	testTyped(t, "float64Slice", []float64(nil), "1.5,2.5", []float64{1.5, 2.5}, "float64Slice")
	testTyped(t, "durationSlice", []time.Duration(nil), "1s,2m", []time.Duration{time.Second, 2 * time.Minute}, "durationSlice")
	testTyped(t, "bytesHex", []byte(nil), "68656c6c6f", []byte("hello"), "bytesHex")
	testTyped(t, "bytesBase64", []byte(nil), "aGVsbG8=", []byte("hello"), "bytesBase64", Base64())
	testTyped(t, "count", int(0), "3", int(3), "count", Count())
	testTyped(t, "ip", net.IP(nil), "192.0.2.1", net.ParseIP("192.0.2.1"), "ip")
	testTyped(t, "ipMask", net.IPMask(nil), "255.255.255.0", ParseIPv4Mask("255.255.255.0"), "ipMask")
	testTyped(t, "ipNet", net.IPNet{}, "192.0.2.0/24", ipnet("192.0.2.0/24"), "ipNet")
	testTyped(t, "ipSlice", []net.IP(nil), "192.0.2.1,192.0.2.2", []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")}, "ipSlice")
	testTyped(t, "ipNetSlice", []net.IPNet(nil), "192.0.2.0/24,198.51.100.0/24", []net.IPNet{ipnet("192.0.2.0/24"), ipnet("198.51.100.0/24")}, "ipNetSlice")
	testTyped(t, "stringToInt", map[string]int(nil), "a=1,b=2", map[string]int{"a": 1, "b": 2}, "stringToInt")
	testTyped(t, "stringToInt64", map[string]int64(nil), "a=1,b=2", map[string]int64{"a": 1, "b": 2}, "stringToInt64")
	testTyped(t, "stringToString", map[string]string(nil), "a=1,b=2", map[string]string{"a": "1", "b": "2"}, "stringToString")
}

func testTyped[T any](t *testing.T, name string, def T, input string, want T, typeName string, options ...Option[T]) {
	t.Helper()
	fs := NewFlagSet(name, ContinueOnError)
	tf := fs.New("x", def, "usage", options...)
	if got := tf.Meta().Type; got != typeName {
		t.Fatalf("%s: type = %q, want %q", name, got, typeName)
	}
	if err := fs.Parse([]string{"--x=" + input}); err != nil {
		t.Fatalf("%s: parse %q: %v", name, input, err)
	}
	flag, err := fs.Lookup[T]("x")
	if err != nil {
		t.Fatalf("%s: Lookup: %v", name, err)
	}
	if flag != tf {
		t.Fatalf("%s: Lookup did not return the registered handle", name)
	}
	if got := flag.Get(); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: value = %#v, want %#v", name, got, want)
	}
}

func TestFlagEnv(t *testing.T) {
	t.Setenv("GPFLAG_TEST_PORT", "7070")
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "listen port").Env("GPFLAG_TEST_PORT")

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := port.Get(); got != 7070 {
		t.Fatalf("value = %d, want 7070", got)
	}
	if !port.Meta().Changed {
		t.Fatalf("Changed = false, want true")
	}
	if got := port.EnvName(); got != "GPFLAG_TEST_PORT" {
		t.Fatalf("EnvName = %q", got)
	}
}

func TestFlagEnvCommandLineWins(t *testing.T) {
	t.Setenv("GPFLAG_TEST_PORT", "7070")
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "listen port").Env("GPFLAG_TEST_PORT")

	if err := fs.Parse([]string{"--port=9090"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := port.Get(); got != 9090 {
		t.Fatalf("value = %d, want 9090", got)
	}
}

func TestFlagEnvNotSet(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "listen port").Env("GPFLAG_TEST_UNSET_PORT")

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := port.Get(); got != 8080 {
		t.Fatalf("value = %d, want default 8080", got)
	}
	if port.Meta().Changed {
		t.Fatalf("Changed = true, want false")
	}
}

func TestFlagEnvInvalid(t *testing.T) {
	t.Setenv("GPFLAG_TEST_PORT", "not-a-number")
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "listen port").Env("GPFLAG_TEST_PORT")

	err := fs.Parse(nil)
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !strings.Contains(err.Error(), "GPFLAG_TEST_PORT") {
		t.Fatalf("error %q does not name the environment variable", err)
	}
	if got := port.Get(); got != 8080 {
		t.Fatalf("value changed on error: %d", got)
	}
}

func TestFlagEnvCleared(t *testing.T) {
	t.Setenv("GPFLAG_TEST_PORT", "7070")
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "listen port").Env("GPFLAG_TEST_PORT").Env("")

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := port.Get(); got != 8080 {
		t.Fatalf("value = %d, want default 8080", got)
	}
}

func TestFlagEnvSlice(t *testing.T) {
	t.Setenv("GPFLAG_TEST_TAGS", "a,b")
	fs := NewFlagSet("test", ContinueOnError)
	tags := fs.New("tag", []string{"default"}, "tags").Env("GPFLAG_TEST_TAGS")

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := tags.Get(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("value = %#v, want [a b]", got)
	}
}

func TestFlagEnvSkippedAfterSet(t *testing.T) {
	t.Setenv("GPFLAG_TEST_PORT", "7070")
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "listen port").Env("GPFLAG_TEST_PORT")

	if err := fs.SetText("port", "1234"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := port.Get(); got != 1234 {
		t.Fatalf("value = %d, want 1234", got)
	}
}

func TestFlagEnvParseAll(t *testing.T) {
	t.Setenv("GPFLAG_TEST_PORT", "7070")
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "listen port").Env("GPFLAG_TEST_PORT")

	if err := fs.ParseAll(nil, func(FlagMeta, string) error { return nil }); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := port.Get(); got != 7070 {
		t.Fatalf("value = %d, want 7070", got)
	}
}

func TestFlagBool(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	verbose := fs.New("verbose", false, "verbose output").Short('v')

	if verbose.Meta().NoOptDefVal != "true" {
		t.Fatalf("NoOptDefVal = %q, want true", verbose.Meta().NoOptDefVal)
	}
	if err := fs.Parse([]string{"-v"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !verbose.Get() {
		t.Fatalf("value = false, want true")
	}
}

func TestFlagSliceAppends(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	tags := fs.New("tag", []string{"default"}, "tags")

	if err := fs.Parse([]string{"--tag=a,b", "--tag=c"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := tags.Get(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("value = %#v, want [a b c]", got)
	}
}

func TestFlagCount(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	verbose := fs.New("verbose", int(0), "verbosity", Count()).Short('v')

	if verbose.Meta().NoOptDefVal != "+1" {
		t.Fatalf("NoOptDefVal = %q, want +1", verbose.Meta().NoOptDefVal)
	}
	if err := fs.Parse([]string{"-vvv"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := verbose.Get(); got != 3 {
		t.Fatalf("value = %d, want 3", got)
	}
}

func TestFlagTimeFormats(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	date := fs.New("date", time.Time{}, "date", TimeFormats("2006-01-02"))

	if err := fs.Parse([]string{"--date=2024-01-02"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	if got := date.Get(); !got.Equal(want) {
		t.Fatalf("value = %v, want %v", got, want)
	}
}

func TestFlagShortReplacesOldShorthand(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	tf := fs.New("x", 0, "").Short('a').Short('b')

	if got := fs.ShorthandLookup("a"); got != nil {
		t.Fatalf("old shorthand still registered: %v", got)
	}
	if got := fs.ShorthandLookup("b"); got == nil || got.Name != tf.Meta().Name {
		t.Fatalf("new shorthand not registered")
	}
}

func TestFlagDuplicateShorthandPanics(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	fs.New("a", 0, "").Short('x')

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected a panic")
		}
	}()
	fs.New("b", 0, "").Short('x')
}

func TestFlagUnsupportedTypePanics(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected a panic")
		}
		if !strings.Contains(r.(string), "unsupported") && !strings.Contains(r.(string), "cannot define") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	fs.New("bad", struct{ X int }{}, "bad")
}

func TestFlagCustomText(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	tf := fs.New("name", upperString("default"), "name")

	if err := fs.Parse([]string{"--name=hello"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := tf.Get(); got != upperString("HELLO") {
		t.Fatalf("value = %q, want HELLO", got)
	}
	if got := tf.Meta().Type; got != "gpflag.upperString" {
		t.Fatalf("type = %q, want gpflag.upperString", got)
	}
}

type upperString string

func (u *upperString) UnmarshalText(b []byte) error {
	*u = upperString(strings.ToUpper(string(b)))
	return nil
}

func (u upperString) MarshalText() ([]byte, error) { return []byte(u), nil }

type genericCustomValue struct{ s string }

func (c *genericCustomValue) Set(s string) error              { c.s = s; return nil }
func (c *genericCustomValue) String() string                  { return c.s }
func (c *genericCustomValue) UnmarshalText(text []byte) error { return c.Set(string(text)) }
func (c genericCustomValue) MarshalText() ([]byte, error)     { return []byte(c.s), nil }

func TestFlagCustomValue(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	tf := fs.New("custom", genericCustomValue{s: "default"}, "custom")

	if err := fs.Parse([]string{"--custom=value"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := tf.Get(); got.s != "value" {
		t.Fatalf("value = %#v, want {value}", got)
	}
	if got := tf.Meta().Type; got != "gpflag.genericCustomValue" {
		t.Fatalf("type = %q, want custom", got)
	}
}

func TestFlagTopLevel(t *testing.T) {
	old := CommandLine
	defer func() { CommandLine = old }()
	CommandLine = NewFlagSet("test", ContinueOnError)

	port := New[int]("port", 8080, "listen port")
	if err := CommandLine.Parse([]string{"--port=9090"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := port.Get(); got != 9090 {
		t.Fatalf("value = %d, want 9090", got)
	}
}

func TestFlagMarkDeprecated(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	tf := fs.New("old", "", "old flag").MarkDeprecated("use --new instead")
	tf.MarkShorthandDeprecated("use -n instead")

	if tf.Meta().Deprecated != "use --new instead" {
		t.Fatalf("Deprecated = %q", tf.Meta().Deprecated)
	}
	if tf.Meta().ShorthandDeprecated != "use -n instead" {
		t.Fatalf("ShorthandDeprecated = %q", tf.Meta().ShorthandDeprecated)
	}
	if !tf.Meta().Hidden {
		t.Fatalf("Hidden = false, want true")
	}
}

func TestFlagNoOptDefVal(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	level := fs.New("level", 0, "level").SetNoOptDefVal("5")

	if err := fs.Parse([]string{"--level"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := level.Get(); got != 5 {
		t.Fatalf("value = %d, want 5", got)
	}
}
