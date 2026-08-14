//go:build !windows && !wasm && !js

package raptor

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <stdint.h>

static void* raptor_dlopen(const char* filename, int flags) {
    return dlopen(filename, flags);
}

static void* raptor_dlsym(void* handle, const char* symbol) {
    return dlsym(handle, symbol);
}

static int raptor_dlclose(void* handle) {
    return dlclose(handle);
}

static const char* raptor_dlerror() {
    return dlerror();
}

static uintptr_t raptor_call_proc(void* proc, uintptr_t* args, int argc) {
    switch (argc) {
        case 0: return ((uintptr_t(*)())proc)();
        case 1: return ((uintptr_t(*)(uintptr_t))proc)(args[0]);
        case 2: return ((uintptr_t(*)(uintptr_t, uintptr_t))proc)(args[0], args[1]);
        case 3: return ((uintptr_t(*)(uintptr_t, uintptr_t, uintptr_t))proc)(args[0], args[1], args[2]);
        case 4: return ((uintptr_t(*)(uintptr_t, uintptr_t, uintptr_t, uintptr_t))proc)(args[0], args[1], args[2], args[3]);
        case 5: return ((uintptr_t(*)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t))proc)(args[0], args[1], args[2], args[3], args[4]);
        case 6: return ((uintptr_t(*)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t))proc)(args[0], args[1], args[2], args[3], args[4], args[5]);
        case 7: return ((uintptr_t(*)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t))proc)(args[0], args[1], args[2], args[3], args[4], args[5], args[6]);
        case 8: return ((uintptr_t(*)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t))proc)(args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7]);
        default: return 0;
    }
}
*/
import "C"

func loadDynamicLibrary(path string) (uintptr, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.raptor_dlopen(cPath, C.RTLD_LAZY|C.RTLD_GLOBAL)
	if handle != nil {
		return uintptr(handle), nil
	}

	baseName := filepath.Base(path)
	candidates := []string{
		filepath.Join("bin", baseName),
		filepath.Join(filepath.Dir(os.Args[0]), baseName),
		baseName,
		"lib" + baseName,
		"lib" + baseName + ".so",
		"lib" + baseName + ".so.0",
		"/usr/lib/" + baseName,
		"/usr/lib/lib" + baseName + ".so",
		"/usr/lib/lib" + baseName + ".so.0",
	}
	for _, cand := range candidates {
		cCand := C.CString(cand)
		h := C.raptor_dlopen(cCand, C.RTLD_LAZY|C.RTLD_GLOBAL)
		C.free(unsafe.Pointer(cCand))
		if h != nil {
			return uintptr(h), nil
		}
	}

	errStr := "unknown dlopen error"
	if cErr := C.raptor_dlerror(); cErr != nil {
		errStr = C.GoString(cErr)
	}
	return 0, fmt.Errorf("failed loading dynamic library %q: %s", path, errStr)
}

func getDynamicProcAddress(handle uintptr, name string) (uintptr, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	addr := C.raptor_dlsym(unsafe.Pointer(handle), cName)
	if addr == nil {
		errStr := "symbol not found"
		if cErr := C.raptor_dlerror(); cErr != nil {
			errStr = C.GoString(cErr)
		}
		return 0, fmt.Errorf("%s", errStr)
	}
	return uintptr(addr), nil
}

func callDynamicProc(proc uintptr, args ...uintptr) (uintptr, error) {
	if proc == 0 {
		return 0, fmt.Errorf("cannot call null proc address")
	}
	var cArgs []C.uintptr_t
	for _, a := range args {
		cArgs = append(cArgs, C.uintptr_t(a))
	}
	var argPtr *C.uintptr_t
	if len(cArgs) > 0 {
		argPtr = &cArgs[0]
	}
	res := C.raptor_call_proc(unsafe.Pointer(proc), argPtr, C.int(len(cArgs)))
	return uintptr(res), nil
}

func freeDynamicLibrary(handle uintptr) error {
	C.raptor_dlclose(unsafe.Pointer(handle))
	return nil
}

func createDynamicCallback(fn func(a1, a2, a3, a4 uintptr) uintptr) uintptr {
	return 0
}
