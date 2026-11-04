package gpflag

// BoolFunc registers a callback accepting an optional boolean argument.
func (fs *FlagSet) BoolFunc(name, usage string, fn func(string) error) {
	fs.BoolFuncP(name, "", usage, fn)
}
func (fs *FlagSet) BoolFuncP(name, shorthand, usage string, fn func(string) error) {
	fs.FuncP(name, shorthand, usage, fn)
	info := fs.lookupCore(name)
	info.Type, info.NoOptDefVal, info.boolFlag = "boolfunc", "true", true
}
func BoolFunc(name, usage string, fn func(string) error) { CommandLine.BoolFunc(name, usage, fn) }
func BoolFuncP(name, shorthand, usage string, fn func(string) error) {
	CommandLine.BoolFuncP(name, shorthand, usage, fn)
}
