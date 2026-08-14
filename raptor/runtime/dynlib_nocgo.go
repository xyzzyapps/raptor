//go:build !windows && !wasm && !js && !cgo

package raptor

import "fmt"

func loadDynamicLibrary(path string) (uintptr, error) {
	return 0, fmt.Errorf("FFI dynamic library loading requires CGO on non-Windows platforms")
}

func getDynamicProcAddress(handle uintptr, name string) (uintptr, error) {
	return 0, fmt.Errorf("FFI dynamic symbols require CGO on non-Windows platforms")
}

func callDynamicProc(proc uintptr, args ...uintptr) (uintptr, error) {
	return 0, fmt.Errorf("FFI calls require CGO on non-Windows platforms")
}

func freeDynamicLibrary(handle uintptr) error {
	return nil
}

func createDynamicCallback(fn func(a1, a2, a3, a4 uintptr) uintptr) uintptr {
	return 0
}
