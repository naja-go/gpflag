package gpflag

import (
	goflag "flag"
	"reflect"
	"strings"
	"testing"
)

func TestFlagCoreCloneCallbacks(t *testing.T) {
	for _, method := range []string{"AddFlag", "AddFlagSet"} {
		t.Run(method, func(t *testing.T) {
			source := NewFlagSet("source", ContinueOnError)
			original := source.NewP("tag_list", "t", []string{"default"}, "tags")
			if err := original.SetText("source"); err != nil {
				t.Fatal(err)
			}

			dest := NewFlagSet("destination", ContinueOnError)
			dest.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
				return NormalizedName(strings.ReplaceAll(name, "_", "-"))
			})
			if method == "AddFlag" {
				dest.AddFlag(original)
			} else {
				dest.AddFlagSet(source)
			}
			clone, err := dest.Lookup[[]string]("tag-list")
			if err != nil {
				t.Fatal(err)
			}
			if &clone.inner == &original.inner || clone.inner.flagOps != clone || clone.inner.fs != dest {
				t.Fatal("clone retained its source core, typed owner, or set")
			}
			clone.Short('u').Env("GPFLAG_CORE_CLONE_TAGS").SetAnnotations(map[string][]string{"origin": {"destination"}})
			t.Setenv("GPFLAG_CORE_CLONE_TAGS", "environment")
			if err := dest.Parse(nil); err != nil {
				t.Fatal(err)
			}
			// The source has already changed, but the clone must treat this as its
			// first assignment and replace defaults rather than append to them.
			if !reflect.DeepEqual(clone.Get(), []string{"environment"}) {
				t.Fatalf("clone parsed with source occurrence state: %v", clone.Get())
			}
			if original.Meta().Shorthand != "t" || original.EnvName() != "" || original.Meta().Annotations != nil {
				t.Fatal("clone configuration changed its source metadata")
			}

			third := NewFlagSet("third", ContinueOnError)
			third.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName { return NormalizedName(strings.ToUpper(name)) })
			third.AddFlagSet(dest)
			again, err := third.Lookup[[]string]("TAG-LIST")
			if err != nil {
				t.Fatal(err)
			}
			if again.Meta().Changed || again.Meta().Shorthand != "u" || again.EnvName() != clone.EnvName() || again.Meta().Annotations["origin"][0] != "destination" {
				t.Fatal("cloning a clone used the original registration's metadata")
			}
			if again.inner.flagOps != again || again.inner.fs != third || again.Ptr() != original.Ptr() {
				t.Fatal("cloning a clone lost its owner, set, or shared variable")
			}
			exported := goflag.NewFlagSet("exported", goflag.ContinueOnError)
			third.CopyToGoFlagSet(exported)
			if err := exported.Set("TAG-LIST", "third"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original.Get(), []string{"third"}) || source.NFlag() != 1 || dest.NFlag() != 1 || third.NFlag() != 1 {
				t.Fatal("exported clone callback used the wrong registration")
			}
			if err := again.SetText("tail"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(again.Get(), []string{"third", "tail"}) {
				t.Fatal("clone lost repetition state")
			}
		})
	}
}

func TestImportedFlagCoreClone(t *testing.T) {
	goFlags := goflag.NewFlagSet("go", goflag.ContinueOnError)
	port := goFlags.String("listen_port", "default", "port")
	verbose := goFlags.Bool("verbose", false, "verbose")
	source := NewFlagSet("source", ContinueOnError)
	source.AddGoFlagSet(goFlags)
	dest := NewFlagSet("destination", ContinueOnError)
	dest.SetNormalizeFunc(func(_ *FlagSet, name string) NormalizedName {
		return NormalizedName(strings.ReplaceAll(name, "_", "-"))
	})
	dest.AddFlagSet(source)
	if got, err := dest.Lookup[string]("listen-port"); got != nil || err == nil {
		t.Fatal("imported clone acquired a typed owner")
	}
	if err := dest.SetAnnotation("listen-port", "origin", []string{"destination"}); err != nil {
		t.Fatal(err)
	}

	exported := goflag.NewFlagSet("exported", goflag.ContinueOnError)
	dest.CopyToGoFlagSet(exported)
	if err := exported.Parse([]string{"-listen-port=changed", "-verbose"}); err != nil {
		t.Fatal(err)
	}
	if *port != "changed" || !*verbose || dest.NFlag() != 2 || source.NFlag() != 0 {
		t.Fatal("imported callbacks lost their variable or receiving set")
	}
	if source.lookupCore("listen_port").Name != "listen_port" || source.lookupCore("listen_port").Annotations != nil {
		t.Fatal("imported clone shared metadata")
	}

	third := NewFlagSet("third", ContinueOnError)
	third.AddFlagSet(dest)
	core := third.lookupCore("listen-port")
	imported, ok := core.flagOps.(*importedFlag)
	if !ok || &imported.inner != core || core.fs != third || core.Changed || core.Annotations["origin"][0] != "destination" {
		t.Fatal("imported clone callback captured stale metadata")
	}
	if err := third.SetText("listen-port", "third"); err != nil {
		t.Fatal(err)
	}
	if *port != "third" || third.NFlag() != 1 || dest.NFlag() != 2 || source.NFlag() != 0 {
		t.Fatal("repeated imported cloning lost independent assignment tracking")
	}
}
