//go:build !js || !wasm

package raptor

import (
	"fmt"
	"moarvm-go/engine"
	"path/filepath"
	"strings"
	"unsafe"
)

type ffiLib struct {
	handle uintptr
	funcs  map[string]uintptr
}

type ffiState struct {
	libs        map[string]*ffiLib
	loadedPaths map[string]string
	nextID      int
	vm          moargo.Engine
}

func (in *Interp) registerFFI() {
	st := &ffiState{
		libs:        make(map[string]*ffiLib),
		loadedPaths: make(map[string]string),
		nextID:      1,
	}

	in.Builtins["ffi_load"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("ffi_load requires library path")
		}
		path := filepath.Clean(args[0].String())
		if key, ok := st.loadedPaths[path]; ok {
			return StringValue(key), nil
		}

		handle, err := loadDynamicLibrary(path)
		if err != nil {
			return nil, err
		}

		libKey := fmt.Sprintf("lib_%d", st.nextID)
		st.nextID++
		st.libs[libKey] = &ffiLib{
			handle: handle,
			funcs:  make(map[string]uintptr),
		}
		st.loadedPaths[path] = libKey
		return StringValue(libKey), nil
	}

	in.Builtins["ffi_call"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("ffi_call requires libKey, funcName, returnType, [args]")
		}
		libKey := args[0].String()
		funcName := args[1].String()
		retType := strings.ToLower(args[2].String())

		lib, ok := st.libs[libKey]
		if !ok {
			return nil, fmt.Errorf("unknown library handle %q", libKey)
		}

		procAddr, ok := lib.funcs[funcName]
		if !ok {
			addr, err := getDynamicProcAddress(lib.handle, funcName)
			if err != nil {
				return nil, fmt.Errorf("symbol %q not found in library %q: %w", funcName, libKey, err)
			}
			procAddr = addr
			lib.funcs[funcName] = addr
		}

		var callArgs []uintptr
		var pinnedByteSlices [][]byte

		if len(args) >= 4 && args[3].Type == ValArray {
			for _, a := range args[3].ArrayVal {
				switch a.Type {
				case ValInt:
					callArgs = append(callArgs, uintptr(a.IntVal))
				case ValFloat:
					u := *(*uintptr)(unsafe.Pointer(&a.FloatVal))
					callArgs = append(callArgs, u)
				case ValString:
					cstr := append([]byte(a.StrVal), 0)
					pinnedByteSlices = append(pinnedByteSlices, cstr)
					callArgs = append(callArgs, uintptr(unsafe.Pointer(&cstr[0])))
				case ValNativePtr:
					callArgs = append(callArgs, a.PtrVal)
				case ValBool:
					if a.BoolVal {
						callArgs = append(callArgs, 1)
					} else {
						callArgs = append(callArgs, 0)
					}
				case ValCStruct:
					if a.CStructVal != nil {
						if a.CStructVal.Class != nil && a.CStructVal.Class.TotalSize <= 8 && a.CStructVal.Class.TotalSize > 0 {
							var u64 uint64
							if len(a.CStructVal.Buffer) > 0 {
								copy((*[8]byte)(unsafe.Pointer(&u64))[:], a.CStructVal.Buffer)
							} else if a.CStructVal.Ptr != 0 {
								ptrBytes := unsafe.Slice((*byte)(unsafe.Pointer(a.CStructVal.Ptr)), a.CStructVal.Class.TotalSize)
								copy((*[8]byte)(unsafe.Pointer(&u64))[:], ptrBytes)
							}
							callArgs = append(callArgs, uintptr(u64))
						} else {
							callArgs = append(callArgs, a.CStructVal.Ptr)
						}
					} else {
						callArgs = append(callArgs, a.PtrVal)
					}
				case ValClosure, ValMultiSub:
					closureVal := a
					cb := createDynamicCallback(func(a1, a2, a3, a4 uintptr) uintptr {
						res, err := in.InvokeCallable(closureVal, []*Value{
							NativePtrValue(a1),
							NativePtrValue(a2),
							NativePtrValue(a3),
							NativePtrValue(a4),
						})
						if err != nil || res == nil {
							return 0
						}
						return uintptr(in.toInt(res))
					})
					callArgs = append(callArgs, cb)
				default:
					callArgs = append(callArgs, 0)
				}
			}
		}

		r1, err := callDynamicProc(procAddr, callArgs...)
		_ = err
		_ = pinnedByteSlices // keep alive

		switch retType {
		case "int", "int64", "long":
			return IntValue(int64(r1)), nil
		case "int32":
			return IntValue(int64(int32(r1))), nil
		case "uint", "uint32", "uint64":
			return IntValue(int64(uint64(r1))), nil
		case "bool":
			return BoolValue(r1 != 0), nil
		case "float", "num", "float64", "double":
			return FloatValue(*(*float64)(unsafe.Pointer(&r1))), nil
		case "float32", "num32":
			f32 := *(*float32)(unsafe.Pointer(&r1))
			return FloatValue(float64(f32)), nil
		case "str", "string":
			if r1 == 0 {
				return StringValue(""), nil
			}
			var bytes []byte
			base := r1
			for {
				b := *(*byte)(unsafe.Pointer(base))
				if b == 0 {
					break
				}
				bytes = append(bytes, b)
				base++
			}
			return StringValue(string(bytes)), nil
		case "ptr", "pointer":
			return NativePtrValue(r1), nil
		case "void":
			return NilValue(), nil
		default:
			return IntValue(int64(r1)), nil
		}
	}

	in.Builtins["ffi_bind"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 5 {
			return nil, fmt.Errorf("ffi_bind requires libKey, funcName, retType, [paramTypes], rakuName")
		}
		libKey := args[0].String()
		funcName := args[1].String()
		retType := args[2].String()
		rakuName := args[4].String()

		in.Builtins[rakuName] = func(in *Interp, callArgs []*Value) (*Value, error) {
			fArgs := []*Value{
				StringValue(libKey),
				StringValue(funcName),
				StringValue(retType),
				ArrayValue(callArgs),
			}
			return in.Builtins["ffi_call"](in, fArgs)
		}

		return BoolValue(true), nil
	}

	in.Builtins["ffi_close"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("ffi_close requires library handle")
		}
		libKey := args[0].String()
		lib, ok := st.libs[libKey]
		if !ok {
			return nil, fmt.Errorf("unknown library handle %q", libKey)
		}
		_ = freeDynamicLibrary(lib.handle)
		delete(st.libs, libKey)
		return BoolValue(true), nil
	}

	// Memory buffers map to keep pinned pointers alive
	pinnedBuffers := make(map[uintptr][]byte)

	// ffi_alloc(sizeBytes) -> ptr
	in.Builtins["ffi_alloc"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("ffi_alloc requires size in bytes")
		}
		size := int(in.toInt(args[0]))
		if size <= 0 {
			size = 1
		}
		buf := make([]byte, size)
		ptr := uintptr(unsafe.Pointer(&buf[0]))
		pinnedBuffers[ptr] = buf
		return NativePtrValue(ptr), nil
	}

	// ffi_free(ptr)
	in.Builtins["ffi_free"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValNativePtr {
			return BoolValue(false), nil
		}
		delete(pinnedBuffers, args[0].PtrVal)
		return BoolValue(true), nil
	}

	// ffi_read_uint8(ptr, offset) -> int
	in.Builtins["ffi_read_uint8"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValNativePtr {
			return IntValue(0), nil
		}
		offset := uintptr(0)
		if len(args) >= 2 {
			offset = uintptr(in.toInt(args[1]))
		}
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		val := *(*uint8)(ptr)
		return IntValue(int64(val)), nil
	}

	// ffi_read_uint16(ptr, offset) -> int
	in.Builtins["ffi_read_uint16"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValNativePtr {
			return IntValue(0), nil
		}
		offset := uintptr(0)
		if len(args) >= 2 {
			offset = uintptr(in.toInt(args[1]))
		}
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		val := *(*uint16)(ptr)
		return IntValue(int64(val)), nil
	}

	// ffi_read_int32(ptr, offset) -> int
	in.Builtins["ffi_read_int32"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValNativePtr {
			return IntValue(0), nil
		}
		offset := uintptr(0)
		if len(args) >= 2 {
			offset = uintptr(in.toInt(args[1]))
		}
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		val := *(*int32)(ptr)
		return IntValue(int64(val)), nil
	}

	// ffi_read_int64(ptr, offset) -> int
	in.Builtins["ffi_read_int64"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValNativePtr {
			return IntValue(0), nil
		}
		offset := uintptr(0)
		if len(args) >= 2 {
			offset = uintptr(in.toInt(args[1]))
		}
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		val := *(*int64)(ptr)
		return IntValue(val), nil
	}

	// ffi_read_float64(ptr, offset) -> num
	in.Builtins["ffi_read_float64"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValNativePtr {
			return FloatValue(0), nil
		}
		offset := uintptr(0)
		if len(args) >= 2 {
			offset = uintptr(in.toInt(args[1]))
		}
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		val := *(*float64)(ptr)
		return FloatValue(val), nil
	}

	// ffi_read_str(ptr, offset) -> str (null-terminated C string)
	in.Builtins["ffi_read_str"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValNativePtr {
			return StringValue(""), nil
		}
		offset := uintptr(0)
		if len(args) >= 2 {
			offset = uintptr(in.toInt(args[1]))
		}
		base := args[0].PtrVal + offset
		var bytes []byte
		for {
			b := *(*byte)(unsafe.Pointer(base))
			if b == 0 {
				break
			}
			bytes = append(bytes, b)
			base++
		}
		return StringValue(string(bytes)), nil
	}

	// ffi_write_uint8(ptr, offset, val)
	in.Builtins["ffi_write_uint8"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 || args[0].Type != ValNativePtr {
			return BoolValue(false), nil
		}
		offset := uintptr(in.toInt(args[1]))
		val := uint8(in.toInt(args[2]))
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		*(*uint8)(ptr) = val
		return BoolValue(true), nil
	}

	// ffi_write_uint16(ptr, offset, val)
	in.Builtins["ffi_write_uint16"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 || args[0].Type != ValNativePtr {
			return BoolValue(false), nil
		}
		offset := uintptr(in.toInt(args[1]))
		val := uint16(in.toInt(args[2]))
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		*(*uint16)(ptr) = val
		return BoolValue(true), nil
	}

	// ffi_write_int32(ptr, offset, val)
	in.Builtins["ffi_write_int32"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 || args[0].Type != ValNativePtr {
			return BoolValue(false), nil
		}
		offset := uintptr(in.toInt(args[1]))
		val := int32(in.toInt(args[2]))
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		*(*int32)(ptr) = val
		return BoolValue(true), nil
	}

	// ffi_write_int64(ptr, offset, val)
	in.Builtins["ffi_write_int64"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 || args[0].Type != ValNativePtr {
			return BoolValue(false), nil
		}
		offset := uintptr(in.toInt(args[1]))
		val := in.toInt(args[2])
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		*(*int64)(ptr) = val
		return BoolValue(true), nil
	}

	// ffi_write_float64(ptr, offset, val)
	in.Builtins["ffi_write_float64"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 || args[0].Type != ValNativePtr {
			return BoolValue(false), nil
		}
		offset := uintptr(in.toInt(args[1]))
		val := in.toFloat(args[2])
		ptr := unsafe.Pointer(args[0].PtrVal + offset)
		*(*float64)(ptr) = val
		return BoolValue(true), nil
	}

	// ffi_read_ptr(ptr, offset) -> Pointer
	in.Builtins["ffi_read_ptr"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || (args[0].Type != ValNativePtr && args[0].Type != ValCStruct) {
			return NativePtrValue(0), nil
		}
		base := args[0].PtrVal
		offset := uintptr(0)
		if len(args) >= 2 {
			offset = uintptr(in.toInt(args[1]))
		}
		ptr := unsafe.Pointer(base + offset)
		val := *(*uintptr)(ptr)
		return NativePtrValue(val), nil
	}

	// ffi_write_ptr(ptr, offset, valPtr)
	in.Builtins["ffi_write_ptr"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 || (args[0].Type != ValNativePtr && args[0].Type != ValCStruct) {
			return BoolValue(false), nil
		}
		base := args[0].PtrVal
		offset := uintptr(in.toInt(args[1]))
		valPtr := uintptr(0)
		if args[2].Type == ValNativePtr || args[2].Type == ValCStruct {
			valPtr = args[2].PtrVal
		} else {
			valPtr = uintptr(in.toInt(args[2]))
		}
		ptr := unsafe.Pointer(base + offset)
		*(*uintptr)(ptr) = valPtr
		return BoolValue(true), nil
	}

	// ffi_callback(closure) -> Pointer
	in.Builtins["ffi_callback"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("ffi_callback requires a closure")
		}
		closureVal := args[0]
		cb := createDynamicCallback(func(a1, a2, a3, a4 uintptr) uintptr {
			res, err := in.InvokeCallable(closureVal, []*Value{
				NativePtrValue(a1),
				NativePtrValue(a2),
				NativePtrValue(a3),
				NativePtrValue(a4),
			})
			if err != nil || res == nil {
				return 0
			}
			return uintptr(in.toInt(res))
		})
		return NativePtrValue(cb), nil
	}

	in.Builtins["OpaquePointer"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return NativePtrValue(0), nil
		}
		return NativePtrValue(uintptr(in.toInt(args[0]))), nil
	}
	in.Builtins["Pointer"] = in.Builtins["OpaquePointer"]
}


