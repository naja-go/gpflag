package gpflag

// Func registers a callback receiving the unmodified argument text. Callback
// errors leave the registration unchanged; callback side effects cannot be undone.
func (fs *FlagSet) Func(name, usage string, fn func(string) error) {
	fs.FuncP(name, "", usage, fn)
}
func (fs *FlagSet) FuncP(name, shorthand, usage string, fn func(string) error) {
	if fn == nil {
		panic("gpflag: nil callback")
	}
	fs.NewP(name, shorthand, "", usage, codecOption[string]{codec: actionCodec{fn}, name: "func"})
}

type actionCodec struct{ callback func(string) error }

func (c actionCodec) parse(_ string, text string, _ bool) (string, error) {
	return text, c.callback(text)
}
func (actionCodec) format(string) string { return "" }

func Func(name, usage string, fn func(string) error) { CommandLine.Func(name, usage, fn) }
func FuncP(name, shorthand, usage string, fn func(string) error) {
	CommandLine.FuncP(name, shorthand, usage, fn)
}
