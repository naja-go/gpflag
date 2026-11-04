package gpflag

import (
	"bytes"
	"io"
	"testing"
)

const expectedOutput = `      --long-form[=true|false]    Some description
      --long-form2[=true|false]   Some description
                                    with multiline
  -s, --long-name[=true|false]    Some description
  -t, --long-name2[=true|false]   Some description with
                                    multiline
`

func setUpPFlagSet(buf io.Writer) *FlagSet {
	f := NewFlagSet("test", ExitOnError)
	f.New[bool]("long-form", false, "Some description").Ptr()
	f.New[bool]("long-form2", false, "Some description\n  with multiline").Ptr()
	f.New[bool]("long-name", false, "Some description").Short('s').Ptr()
	f.New[bool]("long-name2", false, "Some description with\n  multiline").Short('t').Ptr()
	f.SetOutput(buf)
	return f
}

func TestPrintUsage(t *testing.T) {
	buf := bytes.Buffer{}
	f := setUpPFlagSet(&buf)
	f.PrintDefaults()
	res := buf.String()
	if res != expectedOutput {
		t.Errorf("Expected \n%s \nActual \n%s", expectedOutput, res)
	}
}

func setUpPFlagSet2(buf io.Writer) *FlagSet {
	f := NewFlagSet("test", ExitOnError)
	f.New[bool]("long-form", false, "Some description").Ptr()
	f.New[bool]("long-form2", false, "Some description\n  with multiline").Ptr()
	f.New[bool]("long-name", false, "Some description").Short('s').Ptr()
	f.New[bool]("long-name2", false, "Some description with\n  multiline").Short('t').Ptr()
	f.New[string]("some-very-long-arg", "test", "Some very long description having break the limit").Short('l').Ptr()
	f.New[string]("other-very-long-arg", "long-default-value", "Some very long description having break the limit").Short('o').Ptr()
	f.New[string]("some-very-long-arg2", "very long default value", "Some very long description\nwith line break\nmultiple").Ptr()
	f.SetOutput(buf)
	return f
}

const expectedOutput2 = `      --long-form[=true|false]       Some description
      --long-form2[=true|false]      Some description
                                       with multiline
  -s, --long-name[=true|false]       Some description
  -t, --long-name2[=true|false]      Some description with
                                       multiline
  -o, --other-very-long-arg string   Some very long description having
                                     break the limit (default
                                     "long-default-value")
  -l, --some-very-long-arg string    Some very long description having
                                     break the limit (default "test")
      --some-very-long-arg2 string   Some very long description
                                     with line break
                                     multiple (default "very long default
                                     value")
`

func TestPrintUsage_2(t *testing.T) {
	buf := bytes.Buffer{}
	f := setUpPFlagSet2(&buf)
	res := f.FlagUsagesWrapped(80)
	if res != expectedOutput2 {
		t.Errorf("Expected \n%s \nActual \n%s", expectedOutput2, res)
	}
}

// A word wider than the description column must sit on its own line.
// Wrapping has to resume for the words after it (#501).
func TestFlagUsagesWrappedContinuesAfterUnbreakableWord(t *testing.T) {
	f := NewFlagSet("example", ContinueOnError)
	f.New[string]("mount", "", "a mount specification e.g. 'type=bind,source=/opt,destination=/hostopt'. The rest of this description is never wrapped.").Ptr()
	got := f.FlagUsagesWrapped(60)
	want := `      --mount string   a mount specification e.g.
                       'type=bind,source=/opt,destination=/hostopt'.
                       The rest of this description is
                       never wrapped.
`
	if got != want {
		t.Errorf("FlagUsagesWrapped(60)\nwant:\n%q\ngot:\n%q", want, got)
	}
}
