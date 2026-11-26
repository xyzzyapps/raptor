//go:build js && wasm

// cc_backend_wasm.go stubs out host C11 compilation when running inside a browser WASM sandbox.
package raptor

import "fmt"

func (in *Interp) evalCC(source string) (*Value, error) {
	return in.Eval(source)
}

// CompileScriptToNativeExe returns an unsupported error when invoked inside WebAssembly.
func CompileScriptToNativeExe(source, outPath string) error {
	return fmt.Errorf("native C11 compilation is not available inside WebAssembly")
}
