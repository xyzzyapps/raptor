package raptor

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"html"
	"math"
	mrand "math/rand"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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
				elements = append(elements, a.materializeArray()...)
			} else if a.Type == ValLazySeq && a.LazySeq() != nil {
				elements = append(elements, a.LazySeq().Items...)
			} else {
				elements = append(elements, escapeValue(a))
			}
		}
		return JunctionValue(JunctionAll, elements), nil
	}

	in.Builtins["any"] = func(in *Interp, args []*Value) (*Value, error) {
		var elements []*Value
		for _, a := range args {
			if a.Type == ValArray {
				elements = append(elements, a.materializeArray()...)
			} else if a.Type == ValLazySeq && a.LazySeq() != nil {
				elements = append(elements, a.LazySeq().Items...)
			} else {
				elements = append(elements, escapeValue(a))
			}
		}
		return JunctionValue(JunctionAny, elements), nil
	}

	in.Builtins["one"] = func(in *Interp, args []*Value) (*Value, error) {
		var elements []*Value
		for _, a := range args {
			if a.Type == ValArray {
				elements = append(elements, a.materializeArray()...)
			} else if a.Type == ValLazySeq && a.LazySeq() != nil {
				elements = append(elements, a.LazySeq().Items...)
			} else {
				elements = append(elements, escapeValue(a))
			}
		}
		return JunctionValue(JunctionOne, elements), nil
	}

	in.Builtins["none"] = func(in *Interp, args []*Value) (*Value, error) {
		var elements []*Value
		for _, a := range args {
			if a.Type == ValArray {
				elements = append(elements, a.materializeArray()...)
			} else if a.Type == ValLazySeq && a.LazySeq() != nil {
				elements = append(elements, a.LazySeq().Items...)
			} else {
				elements = append(elements, escapeValue(a))
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
		if len(args) < 2 || args[0].Type != ValArray {
			return NilValue(), nil
		}
		arr := args[0]
		for _, x := range args[1:] {
			arr.pushValue(x)
		}
		return IntValue(int64(arr.arrayLen())), nil
	}

	in.Builtins["pop"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || args[0].arrayLen() == 0 {
			return NilValue(), nil
		}
		arr := args[0]
		if l := arr.lane(); l != nil && arr.ArrayVal == nil {
			if l.Ints != nil {
				lastIdx := len(l.Ints) - 1
				val := in.tempInt(l.Ints[lastIdx])
				l.Ints = l.Ints[:lastIdx]
				return val, nil
			}
			if l.Floats != nil {
				lastIdx := len(l.Floats) - 1
				val := in.tempFloat(l.Floats[lastIdx])
				l.Floats = l.Floats[:lastIdx]
				return val, nil
			}
			if l.Strs != nil {
				lastIdx := l.Strs.Len() - 1
				val := StringValue(strings.Clone(l.Strs.At(lastIdx)))
				l.Strs.Data = l.Strs.Data[:l.Strs.Offsets[lastIdx]]
				l.Strs.Offsets = l.Strs.Offsets[:lastIdx+1]
				return val, nil
			}
		}
		lastIdx := len(arr.ArrayVal) - 1
		val := arr.ArrayVal[lastIdx]
		arr.ArrayVal = arr.ArrayVal[:lastIdx]
		return val, nil
	}

	in.Builtins["shift"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || args[0].arrayLen() == 0 {
			return NilValue(), nil
		}
		arr := args[0]
		if l := arr.lane(); l != nil && arr.ArrayVal == nil {
			if l.Ints != nil {
				val := in.tempInt(l.Ints[0])
				l.Ints = l.Ints[1:]
				return val, nil
			}
			if l.Floats != nil {
				val := in.tempFloat(l.Floats[0])
				l.Floats = l.Floats[1:]
				return val, nil
			}
		}
		arr.materializeArray()
		val := arr.ArrayVal[0]
		arr.ArrayVal = arr.ArrayVal[1:]
		return val, nil
	}

	in.Builtins["unshift"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValArray {
			return NilValue(), nil
		}
		arr := args[0]
		arr.materializeArray()
		items := make([]*Value, len(args)-1)
		for i, a := range args[1:] {
			items[i] = escapeValue(a)
		}
		arr.ArrayVal = append(items, arr.ArrayVal...)
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
		tape := NewStrTape(len(parts), len(target))
		for _, p := range parts {
			tape.Append(p)
		}
		return StrTapeArrayValue(tape), nil
	}

	in.Builtins["join"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return StringValue(""), nil
		}
		delim := args[0].String()
		if args[1].Type == ValArray && args[1].ArrayVal == nil {
			if ints := args[1].intLane(); ints != nil {
				parts := make([]string, len(ints))
				for i, n := range ints {
					parts[i] = formatIntFast(n)
				}
				return StringValue(strings.Join(parts, delim)), nil
			}
			if floats := args[1].floatLane(); floats != nil {
				parts := make([]string, len(floats))
				for i, f := range floats {
					parts[i] = strconv.FormatFloat(f, 'g', -1, 64)
				}
				return StringValue(strings.Join(parts, delim)), nil
			}
			if strs := args[1].strLane(); strs != nil {
				n := strs.Len()
				parts := make([]string, n)
				for i := 0; i < n; i++ {
					parts[i] = strs.At(i)
				}
				return StringValue(strings.Join(parts, delim)), nil
			}
		}
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
		out := &Value{Type: ValArray}
		var oneArg [1]*Value
		if ints := list.intLane(); ints != nil {
			for _, n := range ints {
				tMark := in.ds.markTemp()
				oneArg[0] = in.tempInt(n)
				val, err := in.InvokeCallable(callable, oneArg[:])
				if err != nil {
					in.ds.rewindTemp(tMark)
					return nil, err
				}
				out.pushValue(val)
				in.ds.rewindTemp(tMark)
			}
			return out, nil
		}
		if floats := list.floatLane(); floats != nil {
			for _, f := range floats {
				tMark := in.ds.markTemp()
				oneArg[0] = in.tempFloat(f)
				val, err := in.InvokeCallable(callable, oneArg[:])
				if err != nil {
					in.ds.rewindTemp(tMark)
					return nil, err
				}
				out.pushValue(val)
				in.ds.rewindTemp(tMark)
			}
			return out, nil
		}
		if strs := list.strLane(); strs != nil {
			cnt := strs.Len()
			for i := 0; i < cnt; i++ {
				tMark := in.ds.markTemp()
				oneArg[0] = StringValue(strs.At(i))
				val, err := in.InvokeCallable(callable, oneArg[:])
				if err != nil {
					in.ds.rewindTemp(tMark)
					return nil, err
				}
				out.pushValue(val)
				in.ds.rewindTemp(tMark)
			}
			return out, nil
		}
		for _, elem := range list.ArrayVal {
			tMark := in.ds.markTemp()
			oneArg[0] = elem
			val, err := in.InvokeCallable(callable, oneArg[:])
			if err != nil {
				in.ds.rewindTemp(tMark)
				return nil, err
			}
			out.pushValue(val)
			in.ds.rewindTemp(tMark)
		}
		return out, nil
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
		var oneArg [1]*Value
		if ints := list.intLane(); ints != nil {
			var res []int64
			for _, n := range ints {
				tMark := in.ds.markTemp()
				oneArg[0] = in.tempInt(n)
				val, err := in.InvokeCallable(callable, oneArg[:])
				if err != nil {
					in.ds.rewindTemp(tMark)
					return nil, err
				}
				if val.IsTrue() {
					res = append(res, n)
				}
				in.ds.rewindTemp(tMark)
			}
			return IntArrayValue(res), nil
		}
		if floats := list.floatLane(); floats != nil {
			var res []float64
			for _, f := range floats {
				tMark := in.ds.markTemp()
				oneArg[0] = in.tempFloat(f)
				val, err := in.InvokeCallable(callable, oneArg[:])
				if err != nil {
					in.ds.rewindTemp(tMark)
					return nil, err
				}
				if val.IsTrue() {
					res = append(res, f)
				}
				in.ds.rewindTemp(tMark)
			}
			return FloatArrayValue(res), nil
		}
		if strs := list.strLane(); strs != nil {
			cnt := strs.Len()
			tape := NewStrTape(cnt/2+1, len(strs.Data)/2+1)
			for i := 0; i < cnt; i++ {
				tMark := in.ds.markTemp()
				s := strs.At(i)
				oneArg[0] = StringValue(s)
				val, err := in.InvokeCallable(callable, oneArg[:])
				if err != nil {
					in.ds.rewindTemp(tMark)
					return nil, err
				}
				if val.IsTrue() {
					tape.Append(s)
				}
				in.ds.rewindTemp(tMark)
			}
			return StrTapeArrayValue(tape), nil
		}
		var result []*Value
		for _, elem := range list.ArrayVal {
			tMark := in.ds.markTemp()
			oneArg[0] = elem
			val, err := in.InvokeCallable(callable, oneArg[:])
			if err != nil {
				in.ds.rewindTemp(tMark)
				return nil, err
			}
			if val.IsTrue() {
				result = append(result, elem)
			}
			in.ds.rewindTemp(tMark)
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
		if list.ArrayVal == nil {
			if ints := list.intLane(); ints != nil {
				cp := make([]int64, len(ints))
				copy(cp, ints)
				SortInt64Fast(cp)
				return IntArrayValue(cp), nil
			}
			if floats := list.floatLane(); floats != nil {
				cp := make([]float64, len(floats))
				copy(cp, floats)
				slices.Sort(cp)
				return FloatArrayValue(cp), nil
			}
			if strs := list.strLane(); strs != nil {
				n := strs.Len()
				tmp := make([]string, n)
				for i := 0; i < n; i++ {
					tmp[i] = strs.At(i)
				}
				slices.Sort(tmp)
				tape := NewStrTape(n, len(strs.Data))
				for _, s := range tmp {
					tape.Append(s)
				}
				return StrTapeArrayValue(tape), nil
			}
		}
		src := list.materializeArray()
		sorted := make([]*Value, len(src))
		copy(sorted, src)
		slices.SortStableFunc(sorted, in.compareValues)
		return ArrayValue(sorted), nil
	}

	// reverse(@list) or reverse($str)
	in.Builtins["reverse"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return ArrayValue(nil), nil
		}
		if args[0].Type == ValString {
			runes := []rune(args[0].StrVal)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return StringValue(string(runes)), nil
		}
		if args[0].Type != ValArray {
			return ArrayValue(nil), nil
		}
		if args[0].ArrayVal == nil {
			if src := args[0].intLane(); src != nil {
				res := make([]int64, len(src))
				for i, n := range src {
					res[len(src)-1-i] = n
				}
				return IntArrayValue(res), nil
			}
			if src := args[0].floatLane(); src != nil {
				res := make([]float64, len(src))
				for i, f := range src {
					res[len(src)-1-i] = f
				}
				return FloatArrayValue(res), nil
			}
			if src := args[0].strLane(); src != nil {
				n := src.Len()
				tape := NewStrTape(n, len(src.Data))
				for i := n - 1; i >= 0; i-- {
					tape.Append(src.At(i))
				}
				return StrTapeArrayValue(tape), nil
			}
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
		n := list.arrayLen()
		if list.Type != ValArray || n == 0 {
			return NilValue(), nil
		}
		var twoArgs [2]*Value
		acc := escapeValue(list.arrayAt(0))
		for i := 1; i < n; i++ {
			tMark := in.ds.markTemp()
			twoArgs[0] = acc
			twoArgs[1] = list.arrayAt(i)
			val, err := in.InvokeCallable(callable, twoArgs[:])
			if err != nil {
				in.ds.rewindTemp(tMark)
				return nil, err
			}
			acc = in.promoteReturn(val, tMark)
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
		cmd := shellExecCommand(cmdStr)
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
		cmd := shellExecCommand(cmdStr)
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
		raw := args[0].String()
		if !utf8.ValidString(raw) {
			n := len(raw)
			start := int(in.toInt(args[1]))
			if start < 0 {
				start = n + start
			}
			if start < 0 {
				start = 0
			}
			if start >= n {
				return StringValue(""), nil
			}
			if len(args) >= 3 {
				length := int(in.toInt(args[2]))
				if length < 0 {
					length = (n + length) - start
				}
				if length < 0 {
					return StringValue(""), nil
				}
				end := start + length
				if end > n {
					end = n
				}
				return StringValue(raw[start:end]), nil
			}
			return StringValue(raw[start:]), nil
		}
		runes := []rune(raw)
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
	in.Builtins["splice"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return ArrayValue(nil), nil
		}
		arr := args[0].materializeArray()
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
				replacements = args[3].materializeArray()
			} else {
				replacements = args[3:]
			}
		}

		newArr := append(append([]*Value(nil), arr[:start]...), append(replacements, arr[start+length:]...)...)
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
		if len(args) < 1 || args[0].Type != ValArray || args[0].arrayLen() == 0 {
			return NilValue(), nil
		}
		return args[0].arrayAt(0), nil
	}

	in.Builtins["tail"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray || args[0].arrayLen() == 0 {
			return ArrayValue(nil), nil
		}
		if args[0].ArrayVal == nil {
			if ints := args[0].intLane(); ints != nil {
				cp := make([]int64, len(ints)-1)
				copy(cp, ints[1:])
				return IntArrayValue(cp), nil
			}
			if floats := args[0].floatLane(); floats != nil {
				cp := make([]float64, len(floats)-1)
				copy(cp, floats[1:])
				return FloatArrayValue(cp), nil
			}
			if strs := args[0].strLane(); strs != nil {
				n := strs.Len()
				tape := NewStrTape(n-1, len(strs.Data))
				for i := 1; i < n; i++ {
					tape.Append(strs.At(i))
				}
				return StrTapeArrayValue(tape), nil
			}
		}
		return ArrayValue(append([]*Value(nil), args[0].ArrayVal[1:]...)), nil
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
		var oneArg [1]*Value
		n := list.arrayLen()
		for i := 0; i < n; i++ {
			tMark := in.ds.markTemp()
			item := list.arrayAt(i)
			oneArg[0] = item
			res, err := in.InvokeCallable(callable, oneArg[:])
			if err == nil && res.IsTrue() {
				in.ds.rewindTemp(tMark)
				return item, nil
			}
			in.ds.rewindTemp(tMark)
		}
		return NilValue(), nil
	}

	in.Builtins["flat"] = func(in *Interp, args []*Value) (*Value, error) {
		var flatList []*Value
		var flatten func(v *Value)
		flatten = func(v *Value) {
			if v.Type == ValArray {
				for _, it := range v.materializeArray() {
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
		for _, v := range args[0].materializeArray() {
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
		a := args[0].materializeArray()
		b := args[1].materializeArray()
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
	in.Builtins["prod"] = in.Builtins["∏"]

	in.Builtins["scalar"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		if args[0].Type == ValArray {
			return IntValue(int64(args[0].arrayLen())), nil
		}
		if args[0].Type == ValHash {
			return IntValue(int64(len(args[0].HashVal))), nil
		}
		return args[0], nil
	}

	in.Builtins["flip"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		runes := []rune(args[0].String())
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return StringValue(string(runes)), nil
	}

	in.Builtins["words"] = func(in *Interp, args []*Value) (*Value, error) {
		s := ""
		if len(args) >= 1 {
			s = args[0].String()
		} else {
			s = in.topicValue().String()
		}
		ws := strings.Fields(s)
		tape := NewStrTape(len(ws), len(s))
		for _, w := range ws {
			tape.Append(w)
		}
		return StrTapeArrayValue(tape), nil
	}

	in.Builtins["lines"] = func(in *Interp, args []*Value) (*Value, error) {
		s := ""
		if len(args) >= 1 {
			s = args[0].String()
		} else {
			s = in.topicValue().String()
		}
		s = strings.TrimRight(s, "\r\n")
		if s == "" {
			return ArrayValue(nil), nil
		}
		raw := strings.Split(s, "\n")
		tape := NewStrTape(len(raw), len(s))
		for _, ln := range raw {
			tape.Append(strings.TrimSuffix(ln, "\r"))
		}
		return StrTapeArrayValue(tape), nil
	}

	in.Builtins["comb"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return ArrayValue(nil), nil
		}
		if len(args) == 1 {
			runes := []rune(args[0].String())
			tape := NewStrTape(len(runes), len(args[0].String()))
			for _, r := range runes {
				tape.Append(string(r))
			}
			return StrTapeArrayValue(tape), nil
		}
		// Supports both Raku $str.comb($pat_or_size) and comb($pat_or_size, $str)
		target := args[0].String()
		spec := args[1]
		if args[0].Type == ValInt {
			spec = args[0]
			target = args[1].String()
		}
		if spec.Type == ValInt {
			chunk := int(spec.IntVal)
			if chunk <= 0 {
				return ArrayValue(nil), nil
			}
			runes := []rune(target)
			tape := NewStrTape((len(runes)+chunk-1)/chunk, len(target))
			for i := 0; i < len(runes); i += chunk {
				end := i + chunk
				if end > len(runes) {
					end = len(runes)
				}
				tape.Append(string(runes[i:end]))
			}
			return StrTapeArrayValue(tape), nil
		}
		matches, err := RegexFindAll(spec.String(), target)
		if err != nil {
			// Try reversed argument order comb($pat, $str)
			matches2, err2 := RegexFindAll(args[0].String(), args[1].String())
			if err2 != nil {
				return nil, err
			}
			matches = matches2
		}
		tape := NewStrTape(len(matches), len(target))
		for _, m := range matches {
			tape.Append(m)
		}
		return StrTapeArrayValue(tape), nil
	}

	in.Builtins["ucfirst"] = in.Builtins["tc"]
	in.Builtins["lcfirst"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		runes := []rune(args[0].String())
		if len(runes) == 0 {
			return StringValue(""), nil
		}
		runes[0] = unicode.ToLower(runes[0])
		return StringValue(string(runes)), nil
	}

	in.Builtins["hex"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		s := strings.TrimSpace(args[0].String())
		s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
		s = strings.ReplaceAll(s, "_", "")
		v, _ := strconv.ParseInt(s, 16, 64)
		return IntValue(v), nil
	}

	in.Builtins["oct"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		s := strings.TrimSpace(args[0].String())
		s = strings.ReplaceAll(s, "_", "")
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			v, _ := strconv.ParseInt(s[2:], 16, 64)
			return IntValue(v), nil
		}
		if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
			v, _ := strconv.ParseInt(s[2:], 2, 64)
			return IntValue(v), nil
		}
		s = strings.TrimPrefix(strings.TrimPrefix(s, "0o"), "0O")
		s = strings.TrimPrefix(s, "0")
		if s == "" {
			return IntValue(0), nil
		}
		v, _ := strconv.ParseInt(s, 8, 64)
		return IntValue(v), nil
	}

	in.Builtins["printf"] = func(in *Interp, args []*Value) (*Value, error) {
		res, err := in.Builtins["sprintf"](in, args)
		if err != nil {
			return nil, err
		}
		fmt.Fprint(in.Stdout, res.String())
		return BoolValue(true), nil
	}

	in.Builtins["note"] = func(in *Interp, args []*Value) (*Value, error) {
		var parts []string
		for _, a := range args {
			parts = append(parts, a.String())
		}
		fmt.Fprintln(in.Stderr, strings.Join(parts, ""))
		return BoolValue(true), nil
	}

	in.Builtins["sleep"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return FloatValue(0), nil
		}
		secs := in.toFloat(args[0])
		if secs > 0 {
			time.Sleep(time.Duration(secs * float64(time.Second)))
		}
		return FloatValue(secs), nil
	}

	in.Builtins["match"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return NilValue(), nil
		}
		// match($pattern, $text)
		pat := args[0].String()
		txt := args[1].String()
		env := in.CurrentEnv
		if env == nil {
			env = in.GlobalEnv
		}
		okVal, err := in.evalRegexMatchWithCaptures(txt, pat, false, env)
		if err != nil {
			return nil, err
		}
		if !okVal.IsTrue() {
			return NilValue(), nil
		}
		if mVal, ok := env.Lookup("$/"); ok {
			return mVal, nil
		}
		return BoolValue(true), nil
	}

	in.Builtins["match_all"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return ArrayValue(nil), nil
		}
		pat := args[0].String()
		txt := args[1].String()
		all, err := RegexFindAll(pat, txt)
		if err != nil {
			return nil, err
		}
		tape := NewStrTape(len(all), len(txt))
		for _, m := range all {
			tape.Append(m)
		}
		return StrTapeArrayValue(tape), nil
	}

	in.Builtins["subst"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return StringValue(""), nil
		}
		// subst($str, $pattern, $replacement [, $global])
		txt := args[0].String()
		pat := args[1].String()
		repl := args[2].String()
		global := true
		if len(args) >= 4 {
			global = args[3].IsTrue()
		}
		re, err := getOrCompileGoRegex(pat)
		if err != nil {
			return nil, err
		}
		if global {
			return StringValue(re.ReplaceAllString(txt, repl)), nil
		}
		loc := re.FindStringSubmatchIndex(txt)
		if loc == nil {
			return StringValue(txt), nil
		}
		expanded := re.ExpandString(nil, repl, txt, loc)
		return StringValue(txt[:loc[0]] + string(expanded) + txt[loc[1]:]), nil
	}

	in.Builtins["eval"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		code := args[0].String()
		env := in.CurrentEnv
		if env == nil {
			env = in.GlobalEnv
		}
		prog, err := ParseProgram(code)
		if err != nil {
			errVal := StringValue(err.Error())
			env.Define("$!", errVal)
			in.GlobalEnv.Define("$!", errVal)
			return NilValue(), nil
		}
		in.ResolveProgram(prog, env == in.GlobalEnv)
		res, err := in.EvalProgram(prog, env)
		if err != nil {
			errVal := StringValue(err.Error())
			env.Define("$!", errVal)
			in.GlobalEnv.Define("$!", errVal)
			return NilValue(), nil
		}
		env.Define("$!", NilValue())
		in.GlobalEnv.Define("$!", NilValue())
		return res, nil
	}
	in.Builtins["EVAL"] = in.Builtins["eval"]

	in.Builtins["sha256"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		sum := sha256.Sum256([]byte(args[0].String()))
		return StringValue(hex.EncodeToString(sum[:])), nil
	}

	in.Builtins["md5"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		sum := md5.Sum([]byte(args[0].String()))
		return StringValue(hex.EncodeToString(sum[:])), nil
	}

	in.Builtins["hmac_sha256"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return StringValue(""), nil
		}
		mac := hmac.New(sha256.New, []byte(args[0].String()))
		mac.Write([]byte(args[1].String()))
		return StringValue(hex.EncodeToString(mac.Sum(nil))), nil
	}

	in.Builtins["base64_encode"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(base64.StdEncoding.EncodeToString([]byte(args[0].String()))), nil
	}

	in.Builtins["base64_decode"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(args[0].String()))
		if err != nil {
			return StringValue(""), nil
		}
		return StringValue(string(dec)), nil
	}

	in.Builtins["uri_encode"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(url.QueryEscape(args[0].String())), nil
	}

	in.Builtins["uri_decode"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		dec, err := url.QueryUnescape(args[0].String())
		if err != nil {
			return args[0], nil
		}
		return StringValue(dec), nil
	}

	in.Builtins["html_escape"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(html.EscapeString(args[0].String())), nil
	}

	in.Builtins["pack"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		tmpl := args[0].String()
		var vals []*Value
		for _, a := range args[1:] {
			if a.Type == ValArray {
				vals = append(vals, a.materializeArray()...)
			} else {
				vals = append(vals, a)
			}
		}
		return packBytes(in, tmpl, vals)
	}

	in.Builtins["unpack"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return ArrayValue(nil), nil
		}
		return unpackBytes(args[0].String(), []byte(args[1].String()))
	}
}

func packBytes(in *Interp, tmpl string, vals []*Value) (*Value, error) {
	var out []byte
	vIdx := 0
	for i := 0; i < len(tmpl); {
		code := tmpl[i]
		i++
		if code == ' ' || code == '\t' {
			continue
		}
		star := false
		count := 1
		if i < len(tmpl) && tmpl[i] == '*' {
			star = true
			i++
		} else if i < len(tmpl) && tmpl[i] >= '0' && tmpl[i] <= '9' {
			j := i
			for j < len(tmpl) && tmpl[j] >= '0' && tmpl[j] <= '9' {
				j++
			}
			count, _ = strconv.Atoi(tmpl[i:j])
			i = j
		}
		switch code {
		case 'x':
			for k := 0; k < count; k++ {
				out = append(out, 0)
			}
		case 'C', 'c':
			limit := count
			if star {
				limit = len(vals) - vIdx
			}
			for k := 0; k < limit && vIdx < len(vals); k++ {
				out = append(out, byte(in.toInt(vals[vIdx])))
				vIdx++
			}
		case 'n':
			limit := count
			if star {
				limit = len(vals) - vIdx
			}
			for k := 0; k < limit && vIdx < len(vals); k++ {
				var b [2]byte
				binary.BigEndian.PutUint16(b[:], uint16(in.toInt(vals[vIdx])))
				out = append(out, b[:]...)
				vIdx++
			}
		case 'v', 'S', 's':
			limit := count
			if star {
				limit = len(vals) - vIdx
			}
			for k := 0; k < limit && vIdx < len(vals); k++ {
				var b [2]byte
				binary.LittleEndian.PutUint16(b[:], uint16(in.toInt(vals[vIdx])))
				out = append(out, b[:]...)
				vIdx++
			}
		case 'N':
			limit := count
			if star {
				limit = len(vals) - vIdx
			}
			for k := 0; k < limit && vIdx < len(vals); k++ {
				var b [4]byte
				binary.BigEndian.PutUint32(b[:], uint32(in.toInt(vals[vIdx])))
				out = append(out, b[:]...)
				vIdx++
			}
		case 'V', 'I', 'L', 'l':
			limit := count
			if star {
				limit = len(vals) - vIdx
			}
			for k := 0; k < limit && vIdx < len(vals); k++ {
				var b [4]byte
				binary.LittleEndian.PutUint32(b[:], uint32(in.toInt(vals[vIdx])))
				out = append(out, b[:]...)
				vIdx++
			}
		case 'Q', 'q':
			limit := count
			if star {
				limit = len(vals) - vIdx
			}
			for k := 0; k < limit && vIdx < len(vals); k++ {
				var b [8]byte
				binary.LittleEndian.PutUint64(b[:], uint64(in.toInt(vals[vIdx])))
				out = append(out, b[:]...)
				vIdx++
			}
		case 'd':
			limit := count
			if star {
				limit = len(vals) - vIdx
			}
			for k := 0; k < limit && vIdx < len(vals); k++ {
				var b [8]byte
				binary.LittleEndian.PutUint64(b[:], math.Float64bits(in.toFloat(vals[vIdx])))
				out = append(out, b[:]...)
				vIdx++
			}
		case 'A', 'a':
			if vIdx < len(vals) {
				s := []byte(vals[vIdx].String())
				vIdx++
				if star {
					out = append(out, s...)
				} else {
					pad := byte(0)
					if code == 'A' {
						pad = ' '
					}
					if len(s) >= count {
						out = append(out, s[:count]...)
					} else {
						out = append(out, s...)
						for k := len(s); k < count; k++ {
							out = append(out, pad)
						}
					}
				}
			}
		case 'H':
			if vIdx < len(vals) {
				hs := vals[vIdx].String()
				vIdx++
				if !star && count < len(hs) {
					hs = hs[:count]
				}
				if len(hs)%2 == 1 {
					hs += "0"
				}
				dec, _ := hex.DecodeString(hs)
				out = append(out, dec...)
			}
		case 'U':
			limit := count
			if star {
				limit = len(vals) - vIdx
			}
			for k := 0; k < limit && vIdx < len(vals); k++ {
				out = append(out, []byte(string(rune(in.toInt(vals[vIdx]))))...)
				vIdx++
			}
		}
	}
	return StringValue(string(out)), nil
}

func unpackBytes(tmpl string, data []byte) (*Value, error) {
	var res []*Value
	off := 0
	for i := 0; i < len(tmpl); {
		code := tmpl[i]
		i++
		if code == ' ' || code == '\t' {
			continue
		}
		star := false
		count := 1
		if i < len(tmpl) && tmpl[i] == '*' {
			star = true
			i++
		} else if i < len(tmpl) && tmpl[i] >= '0' && tmpl[i] <= '9' {
			j := i
			for j < len(tmpl) && tmpl[j] >= '0' && tmpl[j] <= '9' {
				j++
			}
			count, _ = strconv.Atoi(tmpl[i:j])
			i = j
		}
		switch code {
		case 'x':
			off += count
		case 'C':
			for k := 0; (star || k < count) && off < len(data); k++ {
				res = append(res, IntValue(int64(data[off])))
				off++
			}
		case 'c':
			for k := 0; (star || k < count) && off < len(data); k++ {
				res = append(res, IntValue(int64(int8(data[off]))))
				off++
			}
		case 'n':
			for k := 0; (star || k < count) && off+2 <= len(data); k++ {
				res = append(res, IntValue(int64(binary.BigEndian.Uint16(data[off:off+2]))))
				off += 2
			}
		case 'v', 'S':
			for k := 0; (star || k < count) && off+2 <= len(data); k++ {
				res = append(res, IntValue(int64(binary.LittleEndian.Uint16(data[off:off+2]))))
				off += 2
			}
		case 's':
			for k := 0; (star || k < count) && off+2 <= len(data); k++ {
				res = append(res, IntValue(int64(int16(binary.LittleEndian.Uint16(data[off:off+2])))))
				off += 2
			}
		case 'N':
			for k := 0; (star || k < count) && off+4 <= len(data); k++ {
				res = append(res, IntValue(int64(binary.BigEndian.Uint32(data[off:off+4]))))
				off += 4
			}
		case 'V', 'I', 'L':
			for k := 0; (star || k < count) && off+4 <= len(data); k++ {
				res = append(res, IntValue(int64(binary.LittleEndian.Uint32(data[off:off+4]))))
				off += 4
			}
		case 'l':
			for k := 0; (star || k < count) && off+4 <= len(data); k++ {
				res = append(res, IntValue(int64(int32(binary.LittleEndian.Uint32(data[off:off+4])))))
				off += 4
			}
		case 'Q':
			for k := 0; (star || k < count) && off+8 <= len(data); k++ {
				res = append(res, IntValue(int64(binary.LittleEndian.Uint64(data[off:off+8]))))
				off += 8
			}
		case 'q':
			for k := 0; (star || k < count) && off+8 <= len(data); k++ {
				res = append(res, IntValue(int64(binary.LittleEndian.Uint64(data[off:off+8]))))
				off += 8
			}
		case 'd':
			for k := 0; (star || k < count) && off+8 <= len(data); k++ {
				res = append(res, FloatValue(math.Float64frombits(binary.LittleEndian.Uint64(data[off:off+8]))))
				off += 8
			}
		case 'A', 'a':
			end := len(data)
			if !star && off+count < end {
				end = off + count
			}
			if off <= end {
				seg := string(data[off:end])
				if code == 'A' {
					seg = strings.TrimRight(seg, " \x00")
				}
				res = append(res, StringValue(seg))
				off = end
			}
		case 'H':
			end := len(data)
			hs := hex.EncodeToString(data[off:end])
			if !star && count < len(hs) {
				hs = hs[:count]
				off += (count + 1) / 2
			} else {
				off = end
			}
			res = append(res, StringValue(hs))
		case 'U':
			s := string(data[off:])
			k := 0
			for _, r := range s {
				if !star && k >= count {
					break
				}
				res = append(res, IntValue(int64(r)))
				off += len(string(r))
				k++
			}
		}
	}
	return ArrayValue(res), nil
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
