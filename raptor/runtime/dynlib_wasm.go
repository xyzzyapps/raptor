//go:build wasm || js

package raptor

import "fmt"

func loadDynamicLibrary(path string) (uintptr, error) {
	return 0, fmt.Errorf("FFI dynamic library loading not supported on WebAssembly")
}

func getDynamicProcAddress(handle uintptr, name string) (uintptr, error) {
	return 0, fmt.Errorf("FFI dynamic symbols not supported on WebAssembly")
}

func callDynamicProc(proc uintptr, args ...uintptr) (uintptr, error) {
	return 0, fmt.Errorf("FFI calls not supported on WebAssembly")
}

func freeDynamicLibrary(handle uintptr) error {
	return nil
}

func createDynamicCallback(fn func(a1, a2, a3, a4 uintptr) uintptr) uintptr {
	return 0
}
