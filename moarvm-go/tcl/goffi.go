package tcl

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// RegisterGoFunc binds an arbitrary Go function into the Tcl interpreter using reflection.
func RegisterGoFunc(in *Interp, name string, fn any) error {
	val := reflect.ValueOf(fn)
	typ := val.Type()

	if typ.Kind() != reflect.Func {
		return fmt.Errorf("RegisterGoFunc: %q is not a function (got %s)", name, typ.Kind())
	}

	wrapper := func(interp *Interp, args []string) (string, error) {
		numParams := typ.NumIn()
		isVariadic := typ.IsVariadic()

		if !isVariadic && len(args) != numParams {
			return "", fmt.Errorf("wrong # args for %s: expected %d, got %d", name, numParams, len(args))
		}
		if isVariadic && len(args) < numParams-1 {
			return "", fmt.Errorf("wrong # args for %s: expected at least %d, got %d", name, numParams-1, len(args))
		}

		inVals := make([]reflect.Value, len(args))
		for i, argStr := range args {
			var paramType reflect.Type
			if isVariadic && i >= numParams-1 {
				paramType = typ.In(numParams - 1).Elem()
			} else {
				paramType = typ.In(i)
			}

			convVal, err := convertStringToReflectVal(argStr, paramType)
			if err != nil {
				return "", fmt.Errorf("argument %d type mismatch for %s: %w", i+1, name, err)
			}
			inVals[i] = convVal
		}

		results := val.Call(inVals)
		if len(results) == 0 {
			return "", nil
		}

		// Check if last return is error
		lastIdx := len(results) - 1
		lastVal := results[lastIdx]
		if lastVal.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !lastVal.IsNil() {
				return "", lastVal.Interface().(error)
			}
			results = results[:lastIdx]
		}

		if len(results) == 0 {
			return "", nil
		}

		// Format primary result
		primary := results[0].Interface()
		switch v := primary.(type) {
		case string:
			return v, nil
		case []string:
			return strings.Join(v, " "), nil
		default:
			return fmt.Sprintf("%v", v), nil
		}
	}

	in.RegisterCommand(name, wrapper)
	return nil
}

func convertStringToReflectVal(str string, targetType reflect.Type) (reflect.Value, error) {
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(str), nil
	case reflect.Int, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(str, 0, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		if targetType.Kind() == reflect.Int {
			return reflect.ValueOf(int(intVal)), nil
		} else if targetType.Kind() == reflect.Int32 {
			return reflect.ValueOf(int32(intVal)), nil
		}
		return reflect.ValueOf(intVal), nil
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(str, 0, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		if targetType.Kind() == reflect.Uint {
			return reflect.ValueOf(uint(uintVal)), nil
		} else if targetType.Kind() == reflect.Uint32 {
			return reflect.ValueOf(uint32(uintVal)), nil
		}
		return reflect.ValueOf(uintVal), nil
	case reflect.Float32, reflect.Float64:
		fVal, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		if targetType.Kind() == reflect.Float32 {
			return reflect.ValueOf(float32(fVal)), nil
		}
		return reflect.ValueOf(fVal), nil
	case reflect.Bool:
		strLower := strings.ToLower(str)
		if strLower == "1" || strLower == "true" || strLower == "yes" {
			return reflect.ValueOf(true), nil
		}
		if strLower == "0" || strLower == "false" || strLower == "no" {
			return reflect.ValueOf(false), nil
		}
		return reflect.Value{}, fmt.Errorf("invalid boolean string: %q", str)
	case reflect.Slice:
		if targetType.Elem().Kind() == reflect.String {
			items := strings.Fields(str)
			return reflect.ValueOf(items), nil
		}
		return reflect.Value{}, fmt.Errorf("unsupported slice type: %s", targetType)
	default:
		return reflect.Value{}, fmt.Errorf("unsupported parameter type: %s", targetType)
	}
}

// RegisterGoFFI registers the goffi:: commands in the Tcl interpreter.
func RegisterGoFFI(in *Interp) {
	in.RegisterCommand("goffi::call", func(in *Interp, args []string) (string, error) {
		if len(args) < 1 {
			return "", fmt.Errorf("wrong # args: should be \"goffi::call funcName ?arg ...?\"")
		}
		cmdName := args[0]
		cmdArgs := args[1:]
		return in.EvalWords(append([]string{cmdName}, cmdArgs...))
	})
}

