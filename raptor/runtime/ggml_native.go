//go:build !js || !wasm

package raptor

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Optional native ggml C library (ggml.dll / libggml.so). Tensor compute
// stays on the software C-API backend; the probe exposes ggml_time_us and
// ggml_native_available() so NativeCall / ffi_load can take over when the
// real library is on PATH.

type ggmlNativeLib struct {
	mu     sync.Mutex
	tried  bool
	ok     bool
	path   string
	handle uintptr
	timeUs uintptr
}

var ggmlNative = &ggmlNativeLib{}

func ggmlProbeNative() (bool, string) {
	ggmlNative.mu.Lock()
	defer ggmlNative.mu.Unlock()
	if ggmlNative.tried {
		return ggmlNative.ok, ggmlNative.path
	}
	ggmlNative.tried = true
	names := []string{
		filepath.Join("bin", "ggml.dll"),
		filepath.Join("bin", "libggml.dll"),
		filepath.Join("bin", "libggml.so"),
		filepath.Join("bin", "libggml.dylib"),
		"ggml.dll",
		"libggml.dll",
		"libggml.so",
		"libggml.so.0",
		"libggml.dylib",
		"ggml",
	}
	for _, name := range names {
		if _, err := os.Stat(name); err != nil {
			continue
		}
		h, err := loadDynamicLibrary(name)
		if err != nil || h == 0 {
			continue
		}
		timeUs, err1 := getDynamicProcAddress(h, "ggml_time_us")
		initP, err2 := getDynamicProcAddress(h, "ggml_init")
		if (err1 != nil || timeUs == 0) && (err2 != nil || initP == 0) {
			_ = freeDynamicLibrary(h)
			continue
		}
		ggmlNative.handle = h
		ggmlNative.path = name
		ggmlNative.ok = true
		ggmlNative.timeUs = timeUs
		return true, name
	}
	return false, ""
}

func ggmlNativeTimeUs() (int64, bool) {
	ok, _ := ggmlProbeNative()
	if !ok {
		return 0, false
	}
	ggmlNative.mu.Lock()
	proc := ggmlNative.timeUs
	ggmlNative.mu.Unlock()
	if proc == 0 {
		return time.Now().UnixMicro(), true
	}
	r, err := callDynamicProc(proc)
	if err != nil && r == 0 {
		return time.Now().UnixMicro(), true
	}
	return int64(r), true
}
