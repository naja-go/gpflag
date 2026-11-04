package gpflag

import (
	"errors"
	goflag "flag"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLookupMissing(t *testing.T) {
	var fs FlagSet
	got, err := fs.Lookup[[]bool]("missing")
	var missing *NotExistError
	if got != nil || !errors.As(err, &missing) || missing.GetSpecifiedName() != "missing" {
		t.Fatalf("Lookup = %v, %v; want nil and NotExistError", got, err)
	}
}

func TestLookupExactType(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	count := fs.New("count", 2, "count", Count())
	array := fs.New("array", []string{"a,b"}, "array", StringArray())
	bytes := fs.New("bytes", []byte{1, 2}, "bytes", Base64())
	fs.New("duration", time.Second, "duration")
	type distinctInt int
	testLookupMismatch[distinctInt](t, fs, "count")
	testLookupMismatch[[]bool](t, fs, "array")
	testLookupMismatch[string](t, fs, "bytes")
	testLookupMismatch[int64](t, fs, "duration")
	testLookupMismatch[any](t, fs, "count")
	if count.Get() != 2 || count.Meta().Changed || fs.NFlag() != 0 {
		t.Fatal("failed lookup changed the flag")
	}
	type alias = int
	if got, err := fs.Lookup[alias]("count"); err != nil || got != count {
		t.Fatalf("alias Lookup = %v, %v", got, err)
	}
	if got, err := fs.Lookup[[]string]("array"); err != nil || got != array {
		t.Fatalf("array Lookup = %v, %v", got, err)
	}
	if got, err := fs.Lookup[[]byte]("bytes"); err != nil || got != bytes {
		t.Fatalf("bytes Lookup = %v, %v", got, err)
	}
}

func testLookupMismatch[T any](t *testing.T, fs *FlagSet, name string) {
	t.Helper()
	got, err := fs.Lookup[T](name)
	if got != nil || err == nil {
		t.Fatalf("Lookup[%s](%q) = %v, %v", reflect.TypeFor[T](), name, got, err)
	}
	if !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), reflect.TypeFor[T]().String()) {
		t.Fatalf("unhelpful type error: %v", err)
	}
	var missing *NotExistError
	if errors.As(err, &missing) {
		t.Fatalf("existing flag reported missing: %v", err)
	}
}

func TestLookupImportedRegistration(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	goFlags := goflag.NewFlagSet("go", goflag.ContinueOnError)
	goFlags.String("imported", "default", "imported")
	fs.AddGoFlagSet(goFlags)
	testLookupMismatch[string](t, fs, "imported")
}

func TestLookupNormalizedFlagVar(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	var port int
	original := fs.NewVar(&port, "listen_port", 8080, "port")
	fs.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
		return NormalizedName(strings.ReplaceAll(name, "_", "-"))
	})
	if err := fs.Parse([]string{"--listen-port=9090"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"listen_port", "listen-port"} {
		got, err := fs.Lookup[int](name)
		if err != nil {
			t.Fatal(err)
		}
		if got != original || got.Ptr() != &port || got.Get() != 9090 || got.Meta().Name != "listen-port" {
			t.Fatalf("Lookup(%q) lost identity or metadata", name)
		}
	}
}

func TestLookupCustomTypes(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	custom := fs.New("custom", &genericCustomValue{s: "default"}, "custom")
	text := fs.New("text", upperString("default"), "text")
	if err := fs.Parse([]string{"--custom=changed", "--text=hello"}); err != nil {
		t.Fatal(err)
	}
	if got, err := fs.Lookup[*genericCustomValue]("custom"); err != nil || got != custom || got.Get().s != "changed" {
		t.Fatalf("custom Lookup = %v, %v", got, err)
	}
	if got, err := fs.Lookup[upperString]("text"); err != nil || got != text || got.Get() != "HELLO" {
		t.Fatalf("text Lookup = %v, %v", got, err)
	}
}

func TestLookupAddedFlags(t *testing.T) {
	for _, method := range []string{"AddFlagSet", "AddFlag"} {
		t.Run(method, func(t *testing.T) {
			source := NewFlagSet("source", ContinueOnError)
			original := source.New("port", 8080, "port")
			dest := NewFlagSet("destination", ContinueOnError)
			if method == "AddFlagSet" {
				dest.AddFlagSet(source)
			} else {
				dest.AddFlag(original)
			}
			got, err := dest.Lookup[int]("port")
			if err != nil {
				t.Fatal(err)
			}
			again, err := dest.Lookup[int]("port")
			if err != nil || again != got || got == original || got.Ptr() != original.Ptr() {
				t.Fatal("expected a stable independent registration with a shared variable")
			}
			got.Set(9090)
			if dest.NFlag() != 1 || source.NFlag() != 0 || original.Meta().Changed || original.Get() != 9090 {
				t.Fatal("value sharing or independent change tracking failed")
			}
			got.Env("GPFLAG_LOOKUP_PORT")
			if again.EnvName() != "GPFLAG_LOOKUP_PORT" || original.EnvName() != "" {
				t.Fatal("metadata ownership failed")
			}
		})
	}
}

func TestLookupTopLevel(t *testing.T) {
	old := CommandLine
	t.Cleanup(func() { CommandLine = old })
	CommandLine = NewFlagSet("test", ContinueOnError)
	original := New("port", 8080, "port")
	got, err := Lookup[int]("port")
	if err != nil || got != original {
		t.Fatalf("Lookup = %v, %v", got, err)
	}
}
