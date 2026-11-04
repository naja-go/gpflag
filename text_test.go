package gpflag

import "testing"

type textString string

func (s *textString) UnmarshalText(text []byte) error {
	*s = textString(text)
	return nil
}

func TestTextType(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	var value textString
	f.NewVar(&value, "text", textString(""), "text value")

	flag, err := f.Lookup[textString]("text")
	if err != nil {
		t.Fatal("Lookup[textString]:", err)
	}
	if got := flag.Meta().Type; got != "gpflag.textString" {
		t.Errorf("expected type gpflag.textString, got %q", got)
	}
}

func TestText(t *testing.T) {
	f := NewFlagSet("test", ContinueOnError)
	var value textString
	flag := f.NewVar(&value, "text", textString("default"), "text value")

	if err := f.Parse([]string{"--text=hello"}); err != nil {
		t.Fatal(err)
	}
	if value != "hello" {
		t.Errorf("value = %q, want hello", value)
	}

	flag, err := f.Lookup[textString]("text")
	if err != nil {
		t.Fatal("Lookup[textString]:", err)
	}
	got := flag.Get()
	if got != "hello" {
		t.Errorf("Get() = %q, want hello", got)
	}
}
