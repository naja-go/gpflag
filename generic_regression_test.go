package gpflag

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestFlagTypedCollectionMutation(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	fs.New("port", 8080, "port")
	tags := fs.New("tag", []string{"default"}, "tags")
	tags.Set([]string{"replacement"})
	if !reflect.DeepEqual(tags.Get(), []string{"replacement"}) || !tags.Meta().Changed {
		t.Fatal("typed replacement failed")
	}
}

func TestFlagCustomValuePointer(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	value := &genericCustomValue{s: "default"}
	flag := fs.New("custom", value, "custom")
	var bound *genericCustomValue
	boundFlag := fs.NewVar(&bound, "bound", &genericCustomValue{s: "bound default"}, "bound custom")

	if err := fs.Parse([]string{"--custom=changed", "--bound=bound changed"}); err != nil {
		t.Fatal(err)
	}
	if got := flag.Get(); got != value || got.s != "changed" {
		t.Fatalf("custom = %#v, want original pointer with changed value", got)
	}
	if got := boundFlag.Get(); got != bound || got.s != "bound changed" {
		t.Fatalf("bound = %#v, want bound pointer with changed value", got)
	}
}

func TestFlagNilCustomValuePanics(t *testing.T) {
	for _, constructor := range []string{"Flag", "FlagVar"} {
		t.Run(constructor, func(t *testing.T) {
			fs := NewFlagSet("test", ContinueOnError)
			var value *genericCustomValue
			defer func() {
				got := recover()
				message, ok := got.(string)
				if !ok || !strings.Contains(message, "nil *gpflag.genericCustomValue") {
					t.Fatalf("panic = %v, want clear nil custom value error", got)
				}
			}()
			if constructor == "FlagVar" {
				fs.NewVar(&value, "custom", value, "custom")
			} else {
				fs.New("custom", value, "custom")
			}
		})
	}
}

func TestFlagEnvParseAllPrecedence(t *testing.T) {
	for _, apply := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			args []string
		}{
			{name: "environment"},
			{name: "long", args: []string{"--listen-port=9090"}},
			{name: "normalized", args: []string{"--listen_port=9090"}},
			{name: "short", args: []string{"-p9090"}},
			{name: "repeated", args: []string{"-p9000", "--listen-port=9090"}},
		} {
			mode := "collect/"
			if apply {
				mode = "set/"
			}
			t.Run(mode+tc.name, func(t *testing.T) {
				t.Setenv("GPFLAG_TEST_PARSE_ALL_PORT", "7070")
				fs := NewFlagSet("test", ContinueOnError)
				fs.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
					return NormalizedName(strings.ReplaceAll(name, "_", "-"))
				})
				port := fs.New("listen-port", 8080, "port").Short('p').Env("GPFLAG_TEST_PARSE_ALL_PORT")
				var values []string
				err := fs.ParseAll(tc.args, func(flag FlagMeta, value string) error {
					values = append(values, value)
					if apply {
						return fs.SetText(flag.Name, value)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				want := 7070
				if len(tc.args) > 0 {
					want = 8080
					if apply {
						want = 9090
					}
				}
				if got := port.Get(); got != want {
					t.Fatalf("port = %d, want %d", got, want)
				}
				if len(values) != len(tc.args) {
					t.Fatalf("callback values = %v, want one per CLI flag", values)
				}
			})
		}
	}
}

func TestFlagEnvParseAllCallbackError(t *testing.T) {
	t.Setenv("GPFLAG_TEST_PARSE_ALL_PORT", "7070")
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "port").Env("GPFLAG_TEST_PARSE_ALL_PORT")
	want := errors.New("callback failed")
	if err := fs.ParseAll([]string{"--port=9090"}, func(FlagMeta, string) error { return want }); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if port.Get() != 8080 || port.Meta().Changed || fs.NFlag() != 0 {
		t.Fatalf("callback error applied environment or marked the flag as changed")
	}
}

func TestFlagSetTracksChanges(t *testing.T) {
	t.Setenv("GPFLAG_TEST_TYPED_SET_PORT", "7070")
	fs := NewFlagSet("test", ContinueOnError)
	port := fs.New("port", 8080, "port").Env("GPFLAG_TEST_TYPED_SET_PORT")
	if err := port.SetText("invalid"); err == nil {
		t.Fatal("expected invalid value to be rejected")
	}
	if port.Get() != 8080 || port.Meta().Changed || fs.NFlag() != 0 {
		t.Fatal("failed Set changed the flag")
	}
	port.Set(9090)
	if !port.Meta().Changed || fs.NFlag() != 1 {
		t.Fatal("successful Set did not record the changed flag")
	}
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if port.Get() != 9090 {
		t.Fatalf("environment overwrote programmatic Set: got %d", port.Get())
	}
}

func TestFlagEnvVisitOnce(t *testing.T) {
	for _, sorted := range []bool{false, true} {
		fs := NewFlagSet("test", ContinueOnError)
		fs.SortFlags = sorted
		t.Setenv("GPFLAG_TEST_VISIT_PORT", "7070")
		port := fs.New("port", 8080, "port").Env("GPFLAG_TEST_VISIT_PORT")
		for range 2 {
			if err := fs.Parse(nil); err != nil {
				t.Fatal(err)
			}
		}
		var visited []FlagMeta
		fs.Visit(func(flag FlagMeta) { visited = append(visited, flag) })
		if fs.NFlag() != 1 || len(visited) != 1 || visited[0].Name != port.Meta().Name {
			t.Fatalf("NFlag = %d, visited = %v, want port once", fs.NFlag(), visited)
		}
	}
}

func TestFlagGetSharesSliceStorage(t *testing.T) {
	fs := NewFlagSet("test", ContinueOnError)
	tags := fs.New("tag", []string{"default"}, "tags")
	got := tags.Get()
	got[0] = "changed"
	if tags.Get()[0] != "changed" {
		t.Fatal("Get did not share the slice backing array")
	}
}
