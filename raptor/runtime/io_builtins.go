package raptor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FileHandle wraps an os.File for Raku5 IO operations.
type FileHandle struct {
	File    *os.File
	Scanner *bufio.Scanner
}

// registerIOBuiltins registers Perl5-style IO functions as native Go builtins.
func (in *Interp) registerIOBuiltins() {
	handles := make(map[string]*FileHandle)
	nextID := 1

	// open($path, $mode) -> file handle string
	in.Builtins["open"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("open requires at least a file path")
		}
		path := args[0].String()
		mode := "r"
		if len(args) >= 2 {
			mode = args[1].String()
		}

		var flag int
		switch mode {
		case "r", "<":
			flag = os.O_RDONLY
		case "w", ">":
			flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		case "a", ">>":
			flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		case "rw", "+<", "<>":
			flag = os.O_RDWR | os.O_CREATE
		default:
			flag = os.O_RDONLY
		}

		f, err := os.OpenFile(path, flag, 0644)
		if err != nil {
			return nil, fmt.Errorf("open %q failed: %w", path, err)
		}

		key := fmt.Sprintf("fh_%d", nextID)
		nextID++
		handles[key] = &FileHandle{File: f, Scanner: bufio.NewScanner(f)}
		return StringValue(key), nil
	}

	// close($fh)
	in.Builtins["close"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("close requires a file handle")
		}
		key := args[0].String()
		fh, ok := handles[key]
		if !ok {
			return nil, fmt.Errorf("unknown file handle %q", key)
		}
		err := fh.File.Close()
		delete(handles, key)
		if err != nil {
			return nil, err
		}
		return BoolValue(true), nil
	}

	// readline($fh) -> next line or Nil
	in.Builtins["readline"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		key := args[0].String()
		fh, ok := handles[key]
		if !ok {
			return nil, fmt.Errorf("unknown file handle %q", key)
		}
		if fh.Scanner.Scan() {
			return StringValue(fh.Scanner.Text()), nil
		}
		return NilValue(), nil
	}

	// slurp($path) -> entire file contents as string
	in.Builtins["slurp"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("slurp requires a file path")
		}
		content, err := os.ReadFile(args[0].String())
		if err != nil {
			return nil, err
		}
		return StringValue(string(content)), nil
	}

	// spurt($path, $content) -> write entire content to file
	in.Builtins["spurt"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("spurt requires path and content")
		}
		err := os.WriteFile(args[0].String(), []byte(args[1].String()), 0644)
		if err != nil {
			return nil, err
		}
		return BoolValue(true), nil
	}

	// chomp($str) -> remove trailing newline
	in.Builtins["chomp"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		return StringValue(strings.TrimRight(args[0].String(), "\r\n")), nil
	}

	// chop($str) -> remove last character
	in.Builtins["chop"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		r := []rune(args[0].String())
		if len(r) == 0 {
			return StringValue(""), nil
		}
		return StringValue(string(r[:len(r)-1])), nil
	}

	// length($str) -> string length in characters
	in.Builtins["length"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return IntValue(0), nil
		}
		return IntValue(int64(len([]rune(args[0].String())))), nil
	}

	// index($str, $substr) -> position of first occurrence, -1 if not found
	in.Builtins["index"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return IntValue(-1), nil
		}
		idx := strings.Index(args[0].String(), args[1].String())
		return IntValue(int64(idx)), nil
	}

	// sprintf($fmt, @args) -> formatted string
	in.Builtins["sprintf"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return StringValue(""), nil
		}
		fmtStr := args[0].String()
		var fmtArgs []any
		for _, a := range args[1:] {
			switch a.Type {
			case ValInt:
				fmtArgs = append(fmtArgs, a.IntVal)
			case ValFloat:
				fmtArgs = append(fmtArgs, a.FloatVal)
			case ValBool:
				fmtArgs = append(fmtArgs, a.BoolVal)
			default:
				fmtArgs = append(fmtArgs, a.String())
			}
		}
		return StringValue(fmt.Sprintf(fmtStr, fmtArgs...)), nil
	}

	// die($msg) -> fatal error
	in.Builtins["die"] = func(in *Interp, args []*Value) (*Value, error) {
		msg := "Died"
		if len(args) >= 1 {
			msg = args[0].String()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	// warn($msg) -> print to stderr
	in.Builtins["warn"] = func(in *Interp, args []*Value) (*Value, error) {
		var parts []string
		for _, a := range args {
			parts = append(parts, a.String())
		}
		fmt.Fprintln(in.Stderr, strings.Join(parts, ""))
		return NilValue(), nil
	}

	// exit($code) -> exit process
	in.Builtins["exit"] = func(in *Interp, args []*Value) (*Value, error) {
		code := 0
		if len(args) >= 1 {
			code = int(args[0].IntVal)
		}
		os.Exit(code)
		return NilValue(), nil
	}

	// defined($val) -> true if not Nil
	in.Builtins["defined"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		return BoolValue(args[0].Type != ValNil), nil
	}

	// exists(%hash, $key) -> true if key exists in hash
	in.Builtins["exists"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValHash {
			return BoolValue(false), nil
		}
		_, ok := args[0].HashVal[args[1].String()]
		return BoolValue(ok), nil
	}

	// delete(%hash, $key) -> remove key from hash, return old value
	in.Builtins["delete"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 || args[0].Type != ValHash {
			return NilValue(), nil
		}
		key := args[1].String()
		val, ok := args[0].HashVal[key]
		if ok {
			delete(args[0].HashVal, key)
			return val, nil
		}
		return NilValue(), nil
	}
}
