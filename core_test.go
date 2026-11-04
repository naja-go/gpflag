package gpflag

import (
	"errors"
	goflag "flag"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestNewPConstructors(t *testing.T) {
	old := CommandLine
	t.Cleanup(func() { CommandLine = old })
	fs := NewFlagSet("constructors", ContinueOnError)
	CommandLine = fs
	a := NewP("a", "a", 1, "a")
	b := fs.NewP("b", "b", 2, "b")
	var c, d int
	cf := NewVarP(&c, "c", "c", 3, "c")
	df := fs.NewVarP(&d, "d", "d", 4, "d")
	if c != 3 || d != 4 {
		t.Fatal("constructors did not initialize bound variables")
	}
	if err := fs.Parse([]string{"-a5", "-b6", "-c7", "-d8"}); err != nil {
		t.Fatal(err)
	}
	for i, flag := range []*Flag[int]{a, b, cf, df} {
		if flag.Get() != i+5 || !flag.Meta().Changed {
			t.Fatalf("flag %d = %d", i, flag.Get())
		}
		got, err := fs.Lookup[int](flag.Meta().Name)
		if err != nil || got != flag {
			t.Fatal("constructor did not register the actual Flag[T]")
		}
	}
	if cf.Ptr() != &c || df.Ptr() != &d || c != 7 || d != 8 {
		t.Fatal("lost bound variables")
	}
}

func TestFlagMetaSnapshots(t *testing.T) {
	fs := NewFlagSet("snapshots", ContinueOnError)
	annotations := map[string][]string{"key": {"original"}}
	flag := fs.NewP("port", "p", 80, "port").SetAnnotations(annotations)
	annotations["key"][0] = "caller mutation"
	info := flag.Meta()
	info.Name, info.Shorthand, info.Usage = "bad", "x", "bad"
	info.Annotations["key"][0] = "snapshot mutation"
	fs.VisitAll(func(info FlagMeta) { info.Annotations["key"][0] = "visitor mutation" })
	short := fs.ShorthandLookup("p")
	short.Name = "bad"
	short.Annotations["key"][0] = "shorthand mutation"
	if info := flag.Meta(); info.Name != "port" || info.Shorthand != "p" || info.Usage != "port" || info.Annotations["key"][0] != "original" {
		t.Fatalf("metadata leaked: %+v", info)
	}
	flag.Set(90)
	if flag.Meta().DefValue != "80" || flag.String() != "90" {
		t.Fatal("default snapshot changed with current value")
	}
	if got, err := fs.Lookup[int]("port"); err != nil || got != flag {
		t.Fatal("snapshot mutation corrupted lookup")
	}
}

func TestRegistrationFailureIsAtomic(t *testing.T) {
	fs := NewFlagSet("atomic", ContinueOnError)
	first := fs.NewP("first", "a", 1, "first")
	second := fs.NewP("second", "b", 2, "second")
	bound := 99
	for _, define := range []func(){
		func() { fs.NewVarP(&bound, "third", "a", 3, "duplicate shorthand") },
		func() { fs.NewVarP(&bound, "first", "c", 3, "duplicate name") },
		func() { fs.NewVarP(&bound, "third", "é", 3, "non-ASCII shorthand") },
		func() { second.Short('a') },
		func() { fs.NewVar(&bound, "third", 3, "empty option", nil) },
		func() { fs.NewVar((*int)(nil), "third", 3, "nil binding") },
		func() { fs.FuncP("third", "c", "nil callback", nil) },
	} {
		mustPanic(t, define)
	}
	if len(fs.formal) != 2 || len(fs.shorthands) != 2 || bound != 99 || fs.shorthands['a'] != &first.inner || fs.shorthands['b'] != &second.inner {
		t.Fatal("failed definition mutated registrations or the bound variable")
	}
}

func TestNormalizationCollisionIsAtomic(t *testing.T) {
	fs := NewFlagSet("normalization", ContinueOnError)
	left := fs.New("a_b", 1, "left")
	right := fs.New("a-b", 2, "right")
	left.Set(3)
	mustPanic(t, func() {
		fs.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
			return NormalizedName(strings.ReplaceAll(name, "_", "-"))
		})
	})
	if left.Meta().Name != "a_b" || right.Meta().Name != "a-b" || len(fs.formal) != 2 || fs.NFlag() != 1 {
		t.Fatal("failed normalization changed the set")
	}
	if got, err := fs.Lookup[int]("a_b"); err != nil || got != left {
		t.Fatal("failed normalization changed the normalizer")
	}
}

func TestInheritedMetadataAndNormalization(t *testing.T) {
	source := NewFlagSet("source", ContinueOnError)
	original := source.New("listen_port", 80, "port").SetAnnotations(map[string][]string{"key": {"source"}})
	original.Set(90)
	dest := NewFlagSet("dest", ContinueOnError)
	dest.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
		return NormalizedName(strings.ReplaceAll(name, "_", "-"))
	})
	dest.AddFlagSet(source)
	flag, err := dest.Lookup[int]("listen-port")
	if err != nil {
		t.Fatal(err)
	}
	flag.SetAnnotations(map[string][]string{"key": {"dest"}})
	if flag.Meta().Changed || dest.NFlag() != 0 || original.Meta().Name != "listen_port" || original.Meta().Annotations["key"][0] != "source" {
		t.Fatal("inherited metadata was shared")
	}
	if err := dest.Parse([]string{"--listen-port=100"}); err != nil {
		t.Fatal(err)
	}
	if original.Get() != 100 || dest.NFlag() != 1 || source.NFlag() != 1 {
		t.Fatal("inheritance lost value sharing or assignment tracking")
	}
}

func TestTypedAssignmentAndCollectionErrors(t *testing.T) {
	fs := NewFlagSet("collections", ContinueOnError)
	slice := fs.New("slice", []int{1}, "slice")
	slice.Set([]int{2})
	if err := slice.SetText("3"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(slice.Get(), []int{2, 3}) {
		t.Fatal("typed assignment was not the baseline for appending")
	}
	if err := slice.SetText("4,invalid"); err == nil {
		t.Fatal("invalid slice accepted")
	}
	if !reflect.DeepEqual(slice.Get(), []int{2, 3}) {
		t.Fatal("failed slice parse mutated existing data")
	}
	mapping := fs.New("map", map[string]int{"default": 1}, "map")
	if err := mapping.SetText("a=2"); err != nil {
		t.Fatal(err)
	}
	if err := mapping.SetText("b=3,c=invalid"); err == nil {
		t.Fatal("invalid map accepted")
	}
	if !reflect.DeepEqual(mapping.Get(), map[string]int{"a": 2}) {
		t.Fatal("failed map parse mutated existing data")
	}
}

func TestCustomTypedParserAndGoExport(t *testing.T) {
	type number struct{ n int }
	fs := NewFlagSet("custom", ContinueOnError)
	custom := fs.New("custom", number{1}, "custom", WithParser(
		func(_ number, text string, _ bool) (number, error) {
			n, err := strconv.Atoi(text)
			return number{n}, err
		},
		func(value number) string { return strconv.Itoa(value.n) },
	))
	if err := custom.SetText("5"); err != nil || custom.Get().n != 5 {
		t.Fatalf("custom parse: %v", err)
	}
	want := errors.New("rejected")
	checked := fs.New("checked", 7, "checked", WithParser(
		func(_ int, _ string, _ bool) (int, error) { return 99, want }, strconv.Itoa,
	))
	var callback string
	fs.Func("callback", "callback", func(text string) error { callback = text; return nil })
	goFlags := goflag.NewFlagSet("export", goflag.ContinueOnError)
	fs.CopyToGoFlagSet(goFlags)
	if err := goFlags.Set("checked", "99"); !errors.Is(err, want) || checked.Get() != 7 {
		t.Fatal("Go export bypassed custom scalar parsing")
	}
	if err := goFlags.Set("callback", "called"); err != nil || callback != "called" {
		t.Fatal("Go export bypassed callback parsing")
	}
	if err := goFlags.Set("custom", "6"); err != nil || custom.Get().n != 6 {
		t.Fatal("Go export lost the custom value")
	}
}

func TestNilCustomPointerAssignment(t *testing.T) {
	fs := NewFlagSet("nil pointer", ContinueOnError)
	flag := fs.New("custom", &genericCustomValue{s: "default"}, "custom")
	flag.Set(nil)
	if flag.Get() != nil || flag.String() != "<nil>" {
		t.Fatal("nil typed assignment was lost")
	}
	if err := flag.SetText("next"); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("nil pointer parsing error = %v", err)
	}
}

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	fn()
}
