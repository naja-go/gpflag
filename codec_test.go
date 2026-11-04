package gpflag

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestSharedCodecOptions(t *testing.T) {
	fs := NewFlagSet("shared options", ContinueOnError)
	arrayOption := StringArray()
	first := fs.New("first", []string{"default"}, "first", arrayOption)
	second := fs.New("second", []string{"default"}, "second", arrayOption)
	for _, assignment := range []struct {
		flag *Flag[[]string]
		text string
	}{
		{first, "one,two"}, {first, "three"}, {second, "four,five"},
	} {
		if err := assignment.flag.SetText(assignment.text); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(first.Get(), []string{"one,two", "three"}) || !reflect.DeepEqual(second.Get(), []string{"four,five"}) {
		t.Fatal("codec shared occurrence state between flags")
	}

	layouts := []string{"2006-01-02"}
	dateOption := TimeFormats(layouts...)
	layouts[0] = "caller mutation"
	a := fs.New("date-a", time.Time{}, "date", dateOption)
	b := fs.New("date-b", time.Time{}, "date", dateOption)
	if err := a.SetText("2025-01-02"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetText("2026-03-04"); err != nil {
		t.Fatal(err)
	}
	if a.Get().Year() != 2025 || b.Get().Year() != 2026 {
		t.Fatal("time option lost isolated value storage")
	}
	before := b.Get()
	if err := b.SetText("not-a-date"); err == nil || !b.Get().Equal(before) {
		t.Fatal("failed codec parse changed the value")
	}
}

func TestParsingOptionsPreserveCustomFormatter(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		testCodecFormatter(t, 2, "+1", 3, Count(), Count())
	})
	t.Run("array", func(t *testing.T) {
		testCodecFormatter(t, []string{"default"}, "one,two", []string{"one,two"}, StringArray(), StringArray())
	})
	t.Run("time", func(t *testing.T) {
		want := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
		testCodecFormatter(t, time.Time{}, "2025-01-02", want, TimeFormats(time.RFC3339Nano), TimeFormats("2006-01-02"))
	})
}

func testCodecFormatter[T any](t *testing.T, initial T, text string, want T, options ...Option[T]) {
	t.Helper()
	fs := NewFlagSet("formatter", ContinueOnError)
	format := func(value T) string { return "custom: " + fmt.Sprint(value) }
	custom := WithParser(func(current T, _ string, _ bool) (T, error) {
		return current, errors.New("parsing-only option failed to replace the decoder")
	}, format)
	flag := fs.New("value", initial, "value", append([]Option[T]{custom}, options...)...)
	if flag.Meta().DefValue != format(initial) {
		t.Fatal("option replaced the default formatter")
	}
	if err := flag.SetText(text); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(flag.Get(), want) || flag.String() != format(want) {
		t.Fatal("option lost parsing or custom formatting")
	}
}
