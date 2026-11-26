//go:build js && wasm

package raptor

import "fmt"

func (in *Interp) evalCC(source string) (*Value, error) {
	return in.Eval(source)
}

func CompileScriptToNativeExe(source, outPath string) error {
	return fmt.Errorf("native C11 compilation is not available inside WebAssembly")
}
