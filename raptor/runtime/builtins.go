package raptor

import (
	"fmt"
	"math"
	mrand "math/rand"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"time"
)

func (in *Interp) topicValue() *Value {
	if in.CurrentEnv != nil {
		if v, ok := in.CurrentEnv.Lookup("$_"); ok && v != nil {
			return v
		}
	}
	if v, ok := in.GlobalEnv.Lookup("$_"); ok && v != nil {
		return v
	}
	return NilValue()
}

func (in *Interp) registerBuiltins() {
	// Predefined Globals: Environment & Process
	envMap := make(map[string]*Value)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = StringValue(parts[1])
		}
	}
	in.GlobalEnv.Define("%*ENV", HashValue(envMap))
	in.GlobalEnv.Define("%ENV", HashValue(envMap))
	in.GlobalEnv.Define("$*PID", IntValue(int64(os.Getpid())))
	in.GlobalEnv.Define("$$", IntValue(int64(os.Getpid())))
	cwd, _ := os.Getwd()
	in.GlobalEnv.Define("$*CWD", StringValue(cwd))
	in.GlobalEnv.Define("$*OS", StringValue(goruntime.GOOS))
	exe, _ := os.Executable()
	in.GlobalEnv.Define("$*EXECUTABLE", StringValue(exe))
	in.GlobalEnv.Define("$*PROGRAM", StringValue(os.Args[0]))
	in.GlobalEnv.Define("$*PROGRAM-NAME", StringValue(os.Args[0]))
	in.GlobalEnv.Define("$0", StringValue(os.Args[0]))

	// $*RAPTOR runtime object
	raptorHash := map[string]*Value{
		"name":     StringValue("Raptor"),
		"version":  StringValue("1.0.0"),
		"auth":     StringValue("xyzzyapps"),
		"compiler": StringValue("moarvm-go"),
	}
	in.GlobalEnv.Define("$*RAPTOR", HashValue(raptorHash))

	// $*KERNEL OS/Architecture object
	kernelHash := map[string]*Value{
		"name":    StringValue(goruntime.GOOS),
		"arch":    StringValue(goruntime.GOARCH),
		"version": StringValue(goruntime.Version()),
	}
	in.GlobalEnv.Define("$*KERNEL", HashValue(kernelHash))

	// Punctuation variables
	in.GlobalEnv.Define("$?", IntValue(0))
	in.GlobalEnv.Define("$!", NilValue())
	in.GlobalEnv.Define("$_", NilValue())
	in.GlobalEnv.Define("$/", NilValue())

	var argsList []*Value
	if len(os.Args) > 1 {
		for _, a := range os.Args[1:] {
			argsList = append(argsList, StringValue(a))
		}
	}
	in.GlobalEnv.Define("@*ARGS", ArrayValue(argsList))
	in.GlobalEnv.Define("@ARGV", ArrayValue(argsList))

	// Unicode and standard mathematical constants
	in.GlobalEnv.Define("π", FloatValue(math.Pi))
	in.GlobalEnv.Define("pi", FloatValue(math.Pi))
	in.GlobalEnv.Define("τ", FloatValue(2*math.Pi))
	in.GlobalEnv.Define("tau", FloatValue(2*math.Pi))
	in.GlobalEnv.Define("ℯ", FloatValue(math.E))
	in.GlobalEnv.Define("e", FloatValue(math.E))

	in.Builtins["ref"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return StringValue(""), nil
		}
		return StringValue(args[0].RefType()), nil
	}

	in.Builtins["is_ref"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return BoolValue(false), nil
		}
		return BoolValue(args[0].RefType() != ""), nil
	}

	in.Builtins["regex_engine"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) > 0 {
			name := strings.ToLower(args[0].String())
			if name == "samre" || name == "sam" {
				SetRegexEngine(&SamreEngine{})
			} else {
				SetRegexEngine(&GoRegexpEngine{})
			}
		}
		return StringValue(GetRegexEngine().Name()), nil
	}

	in.Builtins["package_symbols"] = func(in *Interp, args []*Value) (*Value, error) {
		pkg := in.CurrentPackage
		if len(args) > 0 && args[0].Type != ValNil {
			pkg = args[0].String()
		}
		stash := in.GetPackage(pkg)
		return HashValue(stash), nil
	}

	in.Builtins["package_get"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return NilValue(), nil
		}
		pkg := args[0].String()
		sym := args[1].String()
		if val, ok := in.GetPackageSymbol(pkg, sym); ok {
			return val, nil
		}
		return NilValue(), nil
	}

	in.Builtins["package_set"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return NilValue(), nil
		}
		pkg := args[0].String()
		sym := args[1].String()
		val := args[2]
		in.SetPackageSymbol(pkg, sym, val)
		return val, nil
	}

	in.Builtins["package_delete"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		pkg := args[0].String()
		sym := args[1].String()
		if stash, ok := in.Packages[pkg]; ok {
			delete(stash, sym)
			delete(stash, "$"+sym)
			delete(stash, "@"+sym)
			delete(stash, "%"+sym)
			delete(stash, "&"+sym)
			noSigil := strings.TrimLeft(sym, "$@%&")
			delete(stash, noSigil)
			in.GlobalEnv.Delete(pkg + "::" + sym)
			in.GlobalEnv.Delete("&" + pkg + "::" + sym)
			in.GlobalEnv.Delete(pkg + "::$" + sym)
			in.GlobalEnv.Delete("$" + pkg + "::" + sym)
			in.GlobalEnv.Delete(pkg + "::" + noSigil)
			in.GlobalEnv.Delete("$" + pkg + "::" + noSigil)
			return BoolValue(true), nil
		}
		return BoolValue(false), nil
	}

	in.Builtins["all"] = func(in *Interp, args []*Value) (*Value, error) {
		var elements []*Value
		for _, a := range args {
			if a.Type == ValArray {
				elements = append(elements, a.ArrayVal...)
			} else if a.Type == ValLazySeq && a.LazySeqVal != nil {
				elements = append(elements, a.LazySeqVal.Items...)
			} else {
				elements = append(elements, a)
			}
		}
		return JunctionValue(JunctionAll, elements), nil
	}

	in.Builtins["any"] = func(in *Interp, args []*Value) (*Value, error) {
		var elements []*Value
		for _, a := range args {
			if a.Type == ValArray {
				elements = append(elements, a.ArrayVal...)
			} else if a.Type == ValLazySeq && a.LazySeqVal != nil {
				elements = append(elements, a.LazySeqVal.Items...)
			} else {
				elements = append(elements, a)
			}
		}
		return JunctionValue(JunctionAny, elements), nil
	}

	in.Builtins["one"] = func(in *Interp, args []*Value) (*Value, error) {
		var elements []*Value
		for _, a := range args {
			if a.Type == ValArray {
				elements = append(elements, a.ArrayVal...)
			} else if a.Type == ValLazySeq && a.LazySeqVal != nil {
				elements = append(elements, a.LazySeqVal.Items...)
			} else {
				elements = append(elements, a)
			}
		}
		return JunctionValue(JunctionOne, elements), nil
	}

	in.Builtins["none"] = func(in *Interp, args []*Value) (*Value, error) {
		var elements []*Value
		for _, a := range args {
			if a.Type == ValArray {
				elements = append(elements, a.ArrayVal...)
			} else if a.Type == ValLazySeq && a.LazySeqVal != nil {
				elements = append(elements, a.LazySeqVal.Items...)
			} else {
				elements = append(elements, a)
			}
		}
		return JunctionValue(JunctionNone, elements), nil
	}

	in.Builtins["chars"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		return IntValue(int64(len([]rune(args[0].String())))), nil
	}

	in.Builtins["codes"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		var codes []*Value
		for _, r := range []rune(args[0].String()) {
			codes = append(codes, IntValue(int64(r)))
		}
		return ArrayValue(codes), nil
	}

	in.Builtins["tc"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		s := args[0].String()
		if len(s) == 0 {
			return StringValue(""), nil
		}
		runes := []rune(s)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		return StringValue(string(runes)), nil
	}

	in.Builtins["fc"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.ToLower(args[0].String())), nil
	}

	in.Builtins["int"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		return IntValue(in.toInt(args[0])), nil
	}
	in.Builtins["Int"] = in.Builtins["int"]

	in.Builtins["num"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(in.toFloat(args[0])), nil
	}
	in.Builtins["Num"] = in.Builtins["num"]

	in.Builtins["str"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(args[0].String()), nil
	}
	in.Builtins["Str"] = in.Builtins["str"]

	in.Builtins["say"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			fmt.Fprintln(in.Stdout, in.topicValue().String())
			return NilValue(), nil
		}
		var parts []string
		for _, a := range args {
			parts = append(parts, a.String())
		}
		fmt.Fprintln(in.Stdout, strings.Join(parts, ""))
		return NilValue(), nil
	}

	in.Builtins["print"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			fmt.Fprint(in.Stdout, in.topicValue().String())
			return NilValue(), nil
		}
		var parts []string
		for _, a := range args {
			parts = append(parts, a.String())
		}
		fmt.Fprint(in.Stdout, strings.Join(parts, ""))
		return NilValue(), nil
	}

	in.Builtins["elems"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		if args[0].Type == ValArray {
			return IntValue(int64(args[0].arrayLen())), nil
		}
		if args[0].Type == ValHash {
			return IntValue(int64(len(args[0].HashVal))), nil
		}
		return IntValue(1), nil
	}

	in.Builtins["push"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("push requires array and at least one element")
		}
		arr := args[0]
		if arr.Type != ValArray {
			return nil, fmt.Errorf("push requires array target")
		}
		for _, x := range args[1:] {
			arr.pushValue(x)
		}
		return IntValue(int64(arr.arrayLen())), nil
	}

	in.Builtins["pop"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || len(args[0].ArrayVal) == 0 {
			return NilValue(), nil
		}
		arr := args[0]
		lastIdx := len(arr.ArrayVal) - 1
		val := arr.ArrayVal[lastIdx]
		arr.ArrayVal = arr.ArrayVal[:lastIdx]
		return val, nil
	}

	in.Builtins["shift"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || len(args[0].ArrayVal) == 0 {
			return NilValue(), nil
		}
		arr := args[0]
		val := arr.ArrayVal[0]
		arr.ArrayVal = arr.ArrayVal[1:]
		return val, nil
	}

	in.Builtins["unshift"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValArray {
			return nil, fmt.Errorf("unshift requires array and elements")
		}
		arr := args[0]
		arr.ArrayVal = append(args[1:], arr.ArrayVal...)
		return IntValue(int64(len(arr.ArrayVal))), nil
	}

	in.Builtins["keys"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValHash {
			return ArrayValue(nil), nil
		}
		var keys []*Value
		for k := range args[0].HashVal {
			keys = append(keys, StringValue(k))
		}
		return ArrayValue(keys), nil
	}

	in.Builtins["values"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValHash {
			return ArrayValue(nil), nil
		}
		var vals []*Value
		for _, v := range args[0].HashVal {
			vals = append(vals, v)
		}
		return ArrayValue(vals), nil
	}

	in.Builtins["kv"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValHash {
			return ArrayValue(nil), nil
		}
		var pairs []*Value
		for k, v := range args[0].HashVal {
			pairs = append(pairs, StringValue(k), v)
		}
		return ArrayValue(pairs), nil
	}

	in.Builtins["split"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return ArrayValue(nil), nil
		}
		delim := args[0].String()
		target := args[1].String()
		parts := strings.Split(target, delim)
		var res []*Value
		for _, p := range parts {
			res = append(res, StringValue(p))
		}
		return ArrayValue(res), nil
	}

	in.Builtins["join"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return StringValue(""), nil
		}
		delim := args[0].String()
		var parts []string
		if args[1].Type == ValArray {
			for _, item := range args[1].ArrayVal {
				parts = append(parts, item.String())
			}
		} else {
			for _, item := range args[1:] {
				parts = append(parts, item.String())
			}
		}
		return StringValue(strings.Join(parts, delim)), nil
	}

	in.Builtins["substr"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return StringValue(""), nil
		}
		str := []rune(args[0].String())
		start := int(in.toInt(args[1]))
		if start < 0 || start > len(str) {
			return StringValue(""), nil
		}
		if len(args) >= 3 {
			length := int(in.toInt(args[2]))
			if start+length > len(str) {
				length = len(str) - start
			}
			return StringValue(string(str[start : start+length])), nil
		}
		return StringValue(string(str[start:])), nil
	}

	in.Builtins["uc"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.ToUpper(args[0].String())), nil
	}

	in.Builtins["lc"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.ToLower(args[0].String())), nil
	}

	in.Builtins["abs"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		if args[0].Type == ValFloat {
			return FloatValue(math.Abs(args[0].FloatVal)), nil
		}
		v := args[0].IntVal
		if v < 0 {
			v = -v
		}
		return IntValue(v), nil
	}

	in.Builtins["sqrt"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Sqrt(in.toFloat(args[0]))), nil
	}

	// map($callable, @list) or @list.map($callable) via UFCS
	in.Builtins["map"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return ArrayValue(nil), nil
		}
		// UFCS: first arg may be the list, second the callable
		var list *Value
		var callable *Value
		if args[0].Type == ValArray {
			list = args[0]
			callable = args[1]
		} else {
			callable = args[0]
			list = args[1]
		}
		if list.Type != ValArray {
			return ArrayValue(nil), nil
		}
		var result []*Value
		for _, elem := range list.ArrayVal {
			val, err := in.InvokeCallable(callable, []*Value{elem})
			if err != nil {
				return nil, err
			}
			result = append(result, val)
		}
		return ArrayValue(result), nil
	}

	// grep($callable, @list) or @list.grep($callable)
	in.Builtins["grep"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return ArrayValue(nil), nil
		}
		var list *Value
		var callable *Value
		if args[0].Type == ValArray {
			list = args[0]
			callable = args[1]
		} else {
			callable = args[0]
			list = args[1]
		}
		if list.Type != ValArray {
			return ArrayValue(nil), nil
		}
		var result []*Value
		for _, elem := range list.ArrayVal {
			val, err := in.InvokeCallable(callable, []*Value{elem})
			if err != nil {
				return nil, err
			}
			if val.IsTrue() {
				result = append(result, elem)
			}
		}
		return ArrayValue(result), nil
	}

	// sort(@list) or sort($cmp, @list)
	in.Builtins["sort"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return ArrayValue(nil), nil
		}
		var list *Value
		if args[0].Type == ValArray {
			list = args[0]
		} else if len(args) >= 2 && args[1].Type == ValArray {
			list = args[1]
		} else {
			return ArrayValue(nil), nil
		}
		if list.Ints != nil && list.ArrayVal == nil {
			cp := make([]int64, len(list.Ints))
			copy(cp, list.Ints)
			sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
			return IntArrayValue(cp), nil
		}
		src := list.materializeArray()
		sorted := make([]*Value, len(src))
		copy(sorted, src)
		for i := 1; i < len(sorted); i++ {
			for j := i; j > 0; j-- {
				cmp := in.compareValues(sorted[j-1], sorted[j])
				if cmp <= 0 {
					break
				}
				sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
			}
		}
		return ArrayValue(sorted), nil
	}

	// reverse(@list)
	in.Builtins["reverse"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return ArrayValue(nil), nil
		}
		src := args[0].ArrayVal
		result := make([]*Value, len(src))
		for i, v := range src {
			result[len(src)-1-i] = v
		}
		return ArrayValue(result), nil
	}

	// reduce($callable, @list) or @list.reduce($callable)
	in.Builtins["reduce"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return NilValue(), nil
		}
		var list *Value
		var callable *Value
		if args[0].Type == ValArray {
			list = args[0]
			callable = args[1]
		} else {
			callable = args[0]
			list = args[1]
		}
		if list.Type != ValArray || len(list.ArrayVal) == 0 {
			return NilValue(), nil
		}
		acc := list.ArrayVal[0]
		for i := 1; i < len(list.ArrayVal); i++ {
			val, err := in.InvokeCallable(callable, []*Value{acc, list.ArrayVal[i]})
			if err != nil {
				return nil, err
			}
			acc = val
		}
		return acc, nil
	}

	// Environment & System Builtins
	in.Builtins["getenv"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		val := os.Getenv(args[0].String())
		return StringValue(val), nil
	}

	in.Builtins["setenv"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		k := args[0].String()
		v := args[1].String()
		_ = os.Setenv(k, v)
		if envVal, ok := in.GlobalEnv.Lookup("%*ENV"); ok && envVal.Type == ValHash && envVal.HashVal != nil {
			envVal.HashVal[k] = StringValue(v)
		}
		if envVal, ok := in.GlobalEnv.Lookup("%ENV"); ok && envVal.Type == ValHash && envVal.HashVal != nil {
			envVal.HashVal[k] = StringValue(v)
		}
		return BoolValue(true), nil
	}

	in.Builtins["unsetenv"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		k := args[0].String()
		_ = os.Unsetenv(k)
		if envVal, ok := in.GlobalEnv.Lookup("%*ENV"); ok && envVal.Type == ValHash && envVal.HashVal != nil {
			delete(envVal.HashVal, k)
		}
		if envVal, ok := in.GlobalEnv.Lookup("%ENV"); ok && envVal.Type == ValHash && envVal.HashVal != nil {
			delete(envVal.HashVal, k)
		}
		return BoolValue(true), nil
	}

	in.Builtins["pid"] = func(in *Interp, args []*Value) (*Value, error) {
		return IntValue(int64(os.Getpid())), nil
	}

	in.Builtins["time"] = func(in *Interp, args []*Value) (*Value, error) {
		return IntValue(time.Now().Unix()), nil
	}

	in.Builtins["now"] = func(in *Interp, args []*Value) (*Value, error) {
		return FloatValue(float64(time.Now().UnixNano()) / 1e9), nil
	}

	in.Builtins["time_ms"] = func(in *Interp, args []*Value) (*Value, error) {
		return IntValue(time.Now().UnixMilli()), nil
	}

	in.Builtins["gmtime"] = func(in *Interp, args []*Value) (*Value, error) {
		t := time.Now().UTC()
		res := make(map[string]*Value)
		res["sec"] = IntValue(int64(t.Second()))
		res["min"] = IntValue(int64(t.Minute()))
		res["hour"] = IntValue(int64(t.Hour()))
		res["mday"] = IntValue(int64(t.Day()))
		res["mon"] = IntValue(int64(t.Month()))
		res["year"] = IntValue(int64(t.Year()))
		res["wday"] = IntValue(int64(t.Weekday()))
		res["yday"] = IntValue(int64(t.YearDay()))
		return HashValue(res), nil
	}

	in.Builtins["localtime"] = func(in *Interp, args []*Value) (*Value, error) {
		t := time.Now()
		res := make(map[string]*Value)
		res["sec"] = IntValue(int64(t.Second()))
		res["min"] = IntValue(int64(t.Minute()))
		res["hour"] = IntValue(int64(t.Hour()))
		res["mday"] = IntValue(int64(t.Day()))
		res["mon"] = IntValue(int64(t.Month()))
		res["year"] = IntValue(int64(t.Year()))
		res["wday"] = IntValue(int64(t.Weekday()))
		res["yday"] = IntValue(int64(t.YearDay()))
		return HashValue(res), nil
	}

	in.Builtins["system"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(-1), nil
		}
		cmdStr := args[0].String()
		var cmd *exec.Cmd
		if goruntime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", cmdStr)
		} else {
			cmd = exec.Command("sh", "-c", cmdStr)
		}
		cmd.Stdout = in.Stdout
		cmd.Stderr = in.Stderr
		err := cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return IntValue(int64(exitErr.ExitCode())), nil
			}
			return IntValue(-1), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["qx"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		cmdStr := args[0].String()
		var cmd *exec.Cmd
		if goruntime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", cmdStr)
		} else {
			cmd = exec.Command("sh", "-c", cmdStr)
		}
		out, err := cmd.Output()
		if err != nil {
			return StringValue(""), nil
		}
		return StringValue(string(out)), nil
	}
	in.Builtins["shell"] = in.Builtins["qx"]

	// Filesystem & Path Builtins
	in.Builtins["cwd"] = func(in *Interp, args []*Value) (*Value, error) {
		dir, err := os.Getwd()
		if err != nil {
			return StringValue(""), nil
		}
		return StringValue(dir), nil
	}

	in.Builtins["chdir"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		err := os.Chdir(args[0].String())
		return BoolValue(err == nil), nil
	}

	in.Builtins["mkdir"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		err := os.MkdirAll(args[0].String(), 0755)
		return BoolValue(err == nil), nil
	}

	in.Builtins["rmdir"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		err := os.Remove(args[0].String())
		return BoolValue(err == nil), nil
	}

	in.Builtins["unlink"] = in.Builtins["rmdir"]
	in.Builtins["remove"] = in.Builtins["rmdir"]

	in.Builtins["rename"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		err := os.Rename(args[0].String(), args[1].String())
		return BoolValue(err == nil), nil
	}

	in.Builtins["copy"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		srcData, err := os.ReadFile(args[0].String())
		if err != nil {
			return BoolValue(false), nil
		}
		err = os.WriteFile(args[1].String(), srcData, 0644)
		return BoolValue(err == nil), nil
	}

	in.Builtins["dirname"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue("."), nil
		}
		return StringValue(filepath.Dir(args[0].String())), nil
	}

	in.Builtins["basename"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(filepath.Base(args[0].String())), nil
	}

	in.Builtins["abspath"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		abs, err := filepath.Abs(args[0].String())
		if err != nil {
			return args[0], nil
		}
		return StringValue(abs), nil
	}

	in.Builtins["dir"] = func(in *Interp, args []*Value) (*Value, error) {
		path := "."
		if len(args) >= 1 {
			path = args[0].String()
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return ArrayValue(nil), nil
		}
		var list []*Value
		for _, e := range entries {
			list = append(list, StringValue(e.Name()))
		}
		return ArrayValue(list), nil
	}
	in.Builtins["dir_entries"] = in.Builtins["dir"]

	// Math & Numeric Builtins
	in.Builtins["abs"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		if args[0].Type == ValInt {
			v := args[0].IntVal
			if v < 0 {
				return IntValue(-v), nil
			}
			return args[0], nil
		}
		return FloatValue(math.Abs(in.toFloat(args[0]))), nil
	}

	in.Builtins["sqrt"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Sqrt(in.toFloat(args[0]))), nil
	}

	in.Builtins["sin"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Sin(in.toFloat(args[0]))), nil
	}

	in.Builtins["cos"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Cos(in.toFloat(args[0]))), nil
	}

	in.Builtins["tan"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Tan(in.toFloat(args[0]))), nil
	}

	in.Builtins["asin"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Asin(in.toFloat(args[0]))), nil
	}

	in.Builtins["acos"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Acos(in.toFloat(args[0]))), nil
	}

	in.Builtins["atan"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Atan(in.toFloat(args[0]))), nil
	}

	in.Builtins["atan2"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Atan2(in.toFloat(args[0]), in.toFloat(args[1]))), nil
	}

	in.Builtins["log"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Log(in.toFloat(args[0]))), nil
	}

	in.Builtins["log10"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Log10(in.toFloat(args[0]))), nil
	}

	in.Builtins["exp"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		return FloatValue(math.Exp(in.toFloat(args[0]))), nil
	}

	in.Builtins["floor"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		return IntValue(int64(math.Floor(in.toFloat(args[0])))), nil
	}

	in.Builtins["ceil"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		return IntValue(int64(math.Ceil(in.toFloat(args[0])))), nil
	}

	in.Builtins["round"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		return IntValue(int64(math.Round(in.toFloat(args[0])))), nil
	}

	in.Builtins["min"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return NilValue(), nil
		}
		m := args[0]
		for _, a := range args[1:] {
			if in.compareValues(a, m) < 0 {
				m = a
			}
		}
		return m, nil
	}

	in.Builtins["max"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return NilValue(), nil
		}
		m := args[0]
		for _, a := range args[1:] {
			if in.compareValues(a, m) > 0 {
				m = a
			}
		}
		return m, nil
	}

	in.Builtins["clamp"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return NilValue(), nil
		}
		val := args[0]
		low := args[1]
		high := args[2]
		if in.compareValues(val, low) < 0 {
			return low, nil
		}
		if in.compareValues(val, high) > 0 {
			return high, nil
		}
		return val, nil
	}

	in.Builtins["sign"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		f := in.toFloat(args[0])
		if f > 0 {
			return IntValue(1), nil
		}
		if f < 0 {
			return IntValue(-1), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["rand"] = func(in *Interp, args []*Value) (*Value, error) {
		scale := 1.0
		if len(args) >= 1 {
			scale = in.toFloat(args[0])
		}
		return FloatValue(mrand.Float64() * scale), nil
	}

	in.Builtins["srand"] = func(in *Interp, args []*Value) (*Value, error) {
		seed := time.Now().UnixNano()
		if len(args) >= 1 {
			seed = in.toInt(args[0])
		}
		mrand.Seed(seed)
		return NilValue(), nil
	}

	// String Base Functions
	in.Builtins["substr"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return StringValue(""), nil
		}
		runes := []rune(args[0].String())
		start := int(in.toInt(args[1]))
		if start < 0 {
			start = len(runes) + start
		}
		if start < 0 {
			start = 0
		}
		if start >= len(runes) {
			return StringValue(""), nil
		}
		if len(args) >= 3 {
			length := int(in.toInt(args[2]))
			if length < 0 {
				length = (len(runes) + length) - start
			}
			if length < 0 {
				return StringValue(""), nil
			}
			end := start + length
			if end > len(runes) {
				end = len(runes)
			}
			return StringValue(string(runes[start:end])), nil
		}
		return StringValue(string(runes[start:])), nil
	}

	in.Builtins["lc"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.ToLower(args[0].String())), nil
	}

	in.Builtins["uc"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.ToUpper(args[0].String())), nil
	}

	in.Builtins["trim"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.TrimSpace(args[0].String())), nil
	}

	in.Builtins["trim_left"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.TrimLeft(args[0].String(), " \t\r\n")), nil
	}

	in.Builtins["trim_right"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.TrimRight(args[0].String(), " \t\r\n")), nil
	}

	in.Builtins["split"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return ArrayValue(nil), nil
		}
		sep := args[0].String()
		str := args[1].String()
		parts := strings.Split(str, sep)
		var list []*Value
		for _, p := range parts {
			list = append(list, StringValue(p))
		}
		return ArrayValue(list), nil
	}

	in.Builtins["join"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return StringValue(""), nil
		}
		sep := args[0].String()
		var list []*Value
		if args[1].Type == ValArray {
			list = args[1].ArrayVal
		} else {
			list = args[1:]
		}
		var strParts []string
		for _, v := range list {
			strParts = append(strParts, v.String())
		}
		return StringValue(strings.Join(strParts, sep)), nil
	}

	in.Builtins["replace"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return StringValue(""), nil
		}
		str := args[0].String()
		oldS := args[1].String()
		newS := args[2].String()
		return StringValue(strings.ReplaceAll(str, oldS, newS)), nil
	}

	in.Builtins["rindex"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return IntValue(-1), nil
		}
		return IntValue(int64(strings.LastIndex(args[0].String(), args[1].String()))), nil
	}

	in.Builtins["contains"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		return BoolValue(strings.Contains(args[0].String(), args[1].String())), nil
	}

	in.Builtins["starts_with"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		return BoolValue(strings.HasPrefix(args[0].String(), args[1].String())), nil
	}
	in.Builtins["starts-with"] = in.Builtins["starts_with"]

	in.Builtins["ends_with"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		return BoolValue(strings.HasSuffix(args[0].String(), args[1].String())), nil
	}
	in.Builtins["ends-with"] = in.Builtins["ends_with"]

	in.Builtins["ord"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		runes := []rune(args[0].String())
		if len(runes) == 0 {
			return IntValue(0), nil
		}
		return IntValue(int64(runes[0])), nil
	}

	in.Builtins["chr"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(string(rune(in.toInt(args[0])))), nil
	}

	// Array / List Operations
	in.Builtins["push"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValArray {
			return NilValue(), nil
		}
		for _, x := range args[1:] {
			args[0].pushValue(x)
		}
		return IntValue(int64(args[0].arrayLen())), nil
	}

	in.Builtins["pop"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || len(args[0].ArrayVal) == 0 {
			return NilValue(), nil
		}
		last := args[0].ArrayVal[len(args[0].ArrayVal)-1]
		args[0].ArrayVal = args[0].ArrayVal[:len(args[0].ArrayVal)-1]
		return last, nil
	}

	in.Builtins["shift"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || len(args[0].ArrayVal) == 0 {
			return NilValue(), nil
		}
		first := args[0].ArrayVal[0]
		args[0].ArrayVal = args[0].ArrayVal[1:]
		return first, nil
	}

	in.Builtins["unshift"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValArray {
			return NilValue(), nil
		}
		items := args[1:]
		args[0].ArrayVal = append(items, args[0].ArrayVal...)
		return IntValue(int64(len(args[0].ArrayVal))), nil
	}

	in.Builtins["splice"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return ArrayValue(nil), nil
		}
		arr := args[0].ArrayVal
		start := 0
		if len(args) >= 2 {
			start = int(in.toInt(args[1]))
			if start < 0 {
				start = len(arr) + start
			}
			if start < 0 {
				start = 0
			}
			if start > len(arr) {
				start = len(arr)
			}
		}
		length := len(arr) - start
		if len(args) >= 3 {
			length = int(in.toInt(args[2]))
			if length < 0 {
				length = (len(arr) + length) - start
			}
			if length < 0 {
				length = 0
			}
			if start+length > len(arr) {
				length = len(arr) - start
			}
		}

		removed := append([]*Value(nil), arr[start:start+length]...)
		var replacements []*Value
		if len(args) >= 4 {
			if args[3].Type == ValArray {
				replacements = args[3].ArrayVal
			} else {
				replacements = args[3:]
			}
		}

		newArr := append(arr[:start], append(replacements, arr[start+length:]...)...)
		args[0].ArrayVal = newArr
		return ArrayValue(removed), nil
	}

	in.Builtins["elems"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		if args[0].Type == ValArray {
			return IntValue(int64(args[0].arrayLen())), nil
		}
		if args[0].Type == ValHash {
			return IntValue(int64(len(args[0].HashVal))), nil
		}
		return IntValue(1), nil
	}

	in.Builtins["head"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || len(args[0].ArrayVal) == 0 {
			return NilValue(), nil
		}
		return args[0].ArrayVal[0], nil
	}

	in.Builtins["tail"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || len(args[0].ArrayVal) == 0 {
			return ArrayValue(nil), nil
		}
		return ArrayValue(args[0].ArrayVal[1:]), nil
	}

	in.Builtins["first"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return NilValue(), nil
		}
		var list *Value
		var callable *Value
		if args[0].Type == ValArray {
			list = args[0]
			callable = args[1]
		} else {
			callable = args[0]
			list = args[1]
		}
		if list.Type != ValArray {
			return NilValue(), nil
		}
		for _, item := range list.ArrayVal {
			res, err := in.InvokeCallable(callable, []*Value{item})
			if err == nil && res.IsTrue() {
				return item, nil
			}
		}
		return NilValue(), nil
	}

	in.Builtins["flat"] = func(in *Interp, args []*Value) (*Value, error) {
		var flatList []*Value
		var flatten func(v *Value)
		flatten = func(v *Value) {
			if v.Type == ValArray {
				for _, it := range v.ArrayVal {
					flatten(it)
				}
			} else {
				flatList = append(flatList, v)
			}
		}
		for _, a := range args {
			flatten(a)
		}
		return ArrayValue(flatList), nil
	}

	in.Builtins["unique"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return ArrayValue(nil), nil
		}
		seen := make(map[string]bool)
		var uniqueList []*Value
		for _, v := range args[0].ArrayVal {
			s := v.String()
			if !seen[s] {
				seen[s] = true
				uniqueList = append(uniqueList, v)
			}
		}
		return ArrayValue(uniqueList), nil
	}

	in.Builtins["zip"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValArray || args[1].Type != ValArray {
			return ArrayValue(nil), nil
		}
		a := args[0].ArrayVal
		b := args[1].ArrayVal
		minLen := len(a)
		if len(b) < minLen {
			minLen = len(b)
		}
		var zipped []*Value
		for i := 0; i < minLen; i++ {
			zipped = append(zipped, ArrayValue([]*Value{a[i], b[i]}))
		}
		return ArrayValue(zipped), nil
	}

	// Hash Base Functions
	in.Builtins["keys"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValHash {
			return ArrayValue(nil), nil
		}
		var kList []*Value
		for k := range args[0].HashVal {
			kList = append(kList, StringValue(k))
		}
		return ArrayValue(kList), nil
	}

	in.Builtins["values"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValHash {
			return ArrayValue(nil), nil
		}
		var vList []*Value
		for _, v := range args[0].HashVal {
			vList = append(vList, v)
		}
		return ArrayValue(vList), nil
	}

	in.Builtins["pairs"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValHash {
			return ArrayValue(nil), nil
		}
		var pList []*Value
		for k, v := range args[0].HashVal {
			pList = append(pList, ArrayValue([]*Value{StringValue(k), v}))
		}
		return ArrayValue(pList), nil
	}

	in.Builtins["kv"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValHash {
			return ArrayValue(nil), nil
		}
		var kvList []*Value
		for k, v := range args[0].HashVal {
			kvList = append(kvList, StringValue(k), v)
		}
		return ArrayValue(kvList), nil
	}

	in.Builtins["invert"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValHash {
			return HashValue(nil), nil
		}
		inverted := make(map[string]*Value)
		for k, v := range args[0].HashVal {
			inverted[v.String()] = StringValue(k)
		}
		return HashValue(inverted), nil
	}

	in.Builtins["merge"] = func(in *Interp, args []*Value) (*Value, error) {
		merged := make(map[string]*Value)
		for _, a := range args {
			if a.Type == ValHash && a.HashVal != nil {
				for k, v := range a.HashVal {
					merged[k] = v
				}
			}
		}
		return HashValue(merged), nil
	}

	// Type Inspection & Coercion
	in.Builtins["type"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue("Nil"), nil
		}
		return StringValue(args[0].TypeName()), nil
	}
	in.Builtins["WHAT"] = in.Builtins["type"]

	in.Builtins["bool"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		return BoolValue(args[0].IsTrue()), nil
	}
	in.Builtins["Bool"] = in.Builtins["bool"]
}

// compareValues provides natural ordering for sort.
func (in *Interp) compareValues(a, b *Value) int {
	if a.Type == ValInt && b.Type == ValInt {
		if a.IntVal < b.IntVal {
			return -1
		}
		if a.IntVal > b.IntVal {
			return 1
		}
		return 0
	}
	if a.Type == ValFloat && b.Type == ValFloat {
		if a.FloatVal < b.FloatVal {
			return -1
		}
		if a.FloatVal > b.FloatVal {
			return 1
		}
		return 0
	}
	as := a.String()
	bs := b.String()
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}
