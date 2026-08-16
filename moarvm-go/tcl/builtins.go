package tcl

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

func (in *Interp) registerBuiltins() {
	in.RegisterCommand("set", builtinSet)
	in.RegisterCommand("unset", builtinUnset)
	in.RegisterCommand("incr", builtinIncr)
	in.RegisterCommand("append", builtinAppend)
	in.RegisterCommand("puts", builtinPuts)
	in.RegisterCommand("proc", builtinProc)
	in.RegisterCommand("return", builtinReturn)
	in.RegisterCommand("if", builtinIf)
	in.RegisterCommand("while", builtinWhile)
	in.RegisterCommand("for", builtinFor)
	in.RegisterCommand("foreach", builtinForeach)
	in.RegisterCommand("switch", builtinSwitch)
	in.RegisterCommand("expr", builtinExpr)
	in.RegisterCommand("list", builtinList)
	in.RegisterCommand("llength", builtinLlength)
	in.RegisterCommand("lindex", builtinLindex)
	in.RegisterCommand("lappend", builtinLappend)
	in.RegisterCommand("lrange", builtinLrange)
	in.RegisterCommand("linsert", builtinLinsert)
	in.RegisterCommand("lreplace", builtinLreplace)
	in.RegisterCommand("lsearch", builtinLsearch)
	in.RegisterCommand("join", builtinJoin)
	in.RegisterCommand("split", builtinSplit)
	in.RegisterCommand("string", builtinString)
	in.RegisterCommand("info", builtinInfo)
	in.RegisterCommand("eval", builtinEval)
	in.RegisterCommand("global", builtinGlobal)
	in.RegisterCommand("concat", builtinConcat)
	in.RegisterCommand("format", builtinFormat)
	in.RegisterCommand("break", builtinBreak)
	in.RegisterCommand("continue", builtinContinue)
	in.RegisterCommand("apply", builtinApply)
	in.RegisterCommand("yield", builtinYield)
	in.RegisterCommand("coroutine", builtinCoroutine)
}

// SplitTclList splits a Tcl list string into constituent element words respecting braces and quotes.
func SplitTclList(s string) []string {
	var items []string
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= len(runes) {
			break
		}

		// Braced element
		if runes[i] == '{' {
			i++
			depth := 1
			var sb strings.Builder
			for i < len(runes) && depth > 0 {
				c := runes[i]
				if c == '{' {
					depth++
					sb.WriteRune(c)
					i++
				} else if c == '}' {
					depth--
					if depth == 0 {
						i++
						break
					}
					sb.WriteRune(c)
					i++
				} else if c == '\\' && i+1 < len(runes) {
					sb.WriteRune(c)
					i++
					sb.WriteRune(runes[i])
					i++
				} else {
					sb.WriteRune(c)
					i++
				}
			}
			items = append(items, sb.String())
			continue
		}

		// Quoted element
		if runes[i] == '"' {
			i++
			var sb strings.Builder
			for i < len(runes) {
				c := runes[i]
				if c == '"' {
					i++
					break
				}
				if c == '\\' && i+1 < len(runes) {
					sb.WriteRune(c)
					i++
					sb.WriteRune(runes[i])
					i++
				} else {
					sb.WriteRune(c)
					i++
				}
			}
			items = append(items, sb.String())
			continue
		}

		// Bare word
		var sb strings.Builder
		for i < len(runes) && !unicode.IsSpace(runes[i]) {
			sb.WriteRune(runes[i])
			i++
		}
		items = append(items, sb.String())
	}
	return items
}

func builtinSet(in *Interp, args []string) (string, error) {
	if len(args) == 1 {
		return in.GetVar(args[0])
	}
	if len(args) == 2 {
		in.SetVar(args[0], args[1])
		return args[1], nil
	}
	return "", fmt.Errorf("wrong # args: should be \"set varName ?newValue?\"")
}

func builtinUnset(in *Interp, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("wrong # args: should be \"unset varName ?varName ...?\"")
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	scope := in.currentScope()
	for _, name := range args {
		delete(scope.vars, name)
		delete(in.globalScope().vars, name)
	}
	return "", nil
}

func builtinIncr(in *Interp, args []string) (string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", fmt.Errorf("wrong # args: should be \"incr varName ?increment?\"")
	}
	varName := args[0]
	increment := int64(1)
	if len(args) == 2 {
		inc, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return "", fmt.Errorf("expected integer but got %q", args[1])
		}
		increment = inc
	}

	curStr, err := in.GetVar(varName)
	var curVal int64
	if err != nil {
		curVal = 0
	} else {
		c, err := strconv.ParseInt(curStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("can't read variable %q: not an integer", varName)
		}
		curVal = c
	}

	newVal := curVal + increment
	resStr := strconv.FormatInt(newVal, 10)
	in.SetVar(varName, resStr)
	return resStr, nil
}

func builtinAppend(in *Interp, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("wrong # args: should be \"append varName ?value ...?\"")
	}
	varName := args[0]
	cur, _ := in.GetVar(varName)
	var sb strings.Builder
	sb.WriteString(cur)
	for _, v := range args[1:] {
		sb.WriteString(v)
	}
	res := sb.String()
	in.SetVar(varName, res)
	return res, nil
}

func builtinPuts(in *Interp, args []string) (string, error) {
	newline := true
	var text string

	if len(args) == 2 && args[0] == "-nonewline" {
		newline = false
		text = args[1]
	} else if len(args) == 1 {
		text = args[0]
	} else {
		return "", fmt.Errorf("wrong # args: should be \"puts ?-nonewline? string\"")
	}

	in.mu.RLock()
	w := in.stdout
	in.mu.RUnlock()

	if newline {
		_, err := fmt.Fprintln(w, text)
		return "", err
	}
	_, err := fmt.Fprint(w, text)
	return "", err
}

func builtinProc(in *Interp, args []string) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("wrong # args: should be \"proc name args body\"")
	}

	name := args[0]
	rawArgs := strings.Fields(args[1])
	body := args[2]

	in.mu.Lock()
	in.procs[name] = &Proc{
		Name: name,
		Args: rawArgs,
		Body: body,
	}
	in.mu.Unlock()

	return "", nil
}

func builtinApply(in *Interp, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("wrong # args: should be \"apply {arglist body} ?arg ...?\"")
	}
	parts := SplitTclList(args[0])
	if len(parts) < 2 {
		return "", fmt.Errorf("apply: lambda must be {arglist body}")
	}
	formals := SplitTclList(parts[0])
	if len(formals) == 1 && formals[0] == "" {
		formals = nil
	}
	body := parts[1]
	actuals := args[1:]
	if len(actuals) != len(formals) {
		return "", fmt.Errorf("wrong # args: should be \"apply {%s %s} %s\"", parts[0], body, strings.Join(formals, " "))
	}

	in.PushScope()
	defer in.PopScope()
	for i, name := range formals {
		in.SetVar(name, actuals[i])
	}
	res, err := in.Eval(body)
	if err != nil {
		if sig, ok := err.(*ReturnSignal); ok {
			return sig.Value, nil
		}
		return "", err
	}
	return res, nil
}

func builtinYield(in *Interp, args []string) (string, error) {
	val := ""
	if len(args) > 0 {
		val = args[0]
	}
	in.mu.RLock()
	name := in.curCoro
	co := in.coros[name]
	in.mu.RUnlock()
	if name == "" || co == nil {
		return "", fmt.Errorf("yield called outside of a coroutine")
	}
	co.fromCo <- goCoroMsg{val: val}
	resume, ok := <-co.toCo
	if !ok {
		return "", fmt.Errorf("coroutine \"%s\" is dead", name)
	}
	return resume, nil
}

func builtinCoroutine(in *Interp, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("wrong # args: should be \"coroutine name command ?arg ...?\"")
	}
	name := args[0]
	cmd := args[1:]

	in.mu.Lock()
	if existing, ok := in.coros[name]; ok && !existing.dead {
		in.mu.Unlock()
		return "", fmt.Errorf("coroutine \"%s\" already exists", name)
	}
	co := &goCoro{
		name:   name,
		toCo:   make(chan string, 1),
		fromCo: make(chan goCoroMsg, 1),
	}
	in.coros[name] = co
	in.mu.Unlock()

	go func() {
		in.mu.Lock()
		prev := in.curCoro
		in.curCoro = name
		in.mu.Unlock()

		res, err := in.EvalWords(cmd)

		in.mu.Lock()
		in.curCoro = prev
		co.dead = true
		in.mu.Unlock()
		co.fromCo <- goCoroMsg{val: res, done: true, err: err}
	}()

	msg := <-co.fromCo
	if msg.err != nil {
		return "", msg.err
	}
	if msg.done {
		in.mu.Lock()
		delete(in.coros, name)
		in.mu.Unlock()
	}
	return msg.val, nil
}

func (in *Interp) resumeCoro(co *goCoro, arg string) (string, error) {
	if co.dead {
		return "", fmt.Errorf("coroutine \"%s\" is dead", co.name)
	}
	co.toCo <- arg
	msg := <-co.fromCo
	if msg.err != nil {
		return "", msg.err
	}
	if msg.done {
		in.mu.Lock()
		delete(in.coros, co.name)
		in.mu.Unlock()
	}
	return msg.val, nil
}

func builtinReturn(in *Interp, args []string) (string, error) {
	var val string
	if len(args) >= 1 {
		val = args[0]
	}
	return val, &ReturnSignal{Value: val}
}

func builtinIf(in *Interp, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("wrong # args: no expression after \"if\" argument")
	}

	i := 0
	for i < len(args) {
		cond := args[i]
		if i+1 >= len(args) {
			return "", fmt.Errorf("wrong # args: missing body after condition %q", cond)
		}
		body := args[i+1]

		// If the keyword is 'then', skip it
		if cond == "then" && i+2 < len(args) {
			cond = args[i+1]
			body = args[i+2]
			i++
		}

		isTrue, err := in.evalCondition(cond)
		if err != nil {
			return "", err
		}

		if isTrue {
			return in.Eval(body)
		}

		i += 2
		if i < len(args) {
			kw := args[i]
			if kw == "elseif" {
				i++ // Move to next condition
				continue
			} else if kw == "else" {
				if i+1 >= len(args) {
					return "", fmt.Errorf("wrong # args: missing body after \"else\"")
				}
				return in.Eval(args[i+1])
			} else {
				// Implicit else body
				return in.Eval(args[i])
			}
		}
	}

	return "", nil
}

func builtinWhile(in *Interp, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("wrong # args: should be \"while test command\"")
	}

	condStr := args[0]
	bodyStr := args[1]
	var lastResult string

	for {
		isTrue, err := in.evalCondition(condStr)
		if err != nil {
			return "", err
		}
		if !isTrue {
			break
		}

		res, err := in.Eval(bodyStr)
		if err != nil {
			if err == ErrBreak {
				break
			}
			if err == ErrContinue {
				continue
			}
			if sig, ok := err.(*ReturnSignal); ok {
				return sig.Value, sig
			}
			return "", err
		}
		lastResult = res
	}

	return lastResult, nil
}

func builtinFor(in *Interp, args []string) (string, error) {
	if len(args) != 4 {
		return "", fmt.Errorf("wrong # args: should be \"for start test next command\"")
	}

	startCmd := args[0]
	testCond := args[1]
	nextCmd := args[2]
	bodyCmd := args[3]

	if _, err := in.Eval(startCmd); err != nil {
		return "", fmt.Errorf("error in for loop start: %w", err)
	}

	var lastResult string
	for {
		isTrue, err := in.evalCondition(testCond)
		if err != nil {
			return "", err
		}
		if !isTrue {
			break
		}

		res, err := in.Eval(bodyCmd)
		if err != nil {
			if err == ErrBreak {
				break
			}
			if err == ErrContinue {
				// execute nextCmd and continue
			} else if sig, ok := err.(*ReturnSignal); ok {
				return sig.Value, sig
			} else {
				return "", err
			}
		} else {
			lastResult = res
		}

		if _, err := in.Eval(nextCmd); err != nil {
			return "", fmt.Errorf("error in for loop next: %w", err)
		}
	}

	return lastResult, nil
}

func builtinForeach(in *Interp, args []string) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("wrong # args: should be \"foreach varName list body\"")
	}

	varName := args[0]
	items := SplitTclList(args[1])
	body := args[2]
	var lastResult string

	for _, item := range items {
		in.SetVar(varName, item)
		res, err := in.Eval(body)
		if err != nil {
			if err == ErrBreak {
				break
			}
			if err == ErrContinue {
				continue
			}
			if sig, ok := err.(*ReturnSignal); ok {
				return sig.Value, sig
			}
			return "", err
		}
		lastResult = res
	}

	return lastResult, nil
}

func builtinSwitch(in *Interp, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("wrong # args: should be \"switch ?options? string pattern body ...\" or \"switch ?options? string {pattern body ...}\"")
	}

	val := args[0]
	var patterns []string
	if len(args) == 2 {
		patterns = SplitTclList(args[1])
	} else {
		patterns = args[1:]
	}

	if len(patterns)%2 != 0 {
		return "", fmt.Errorf("extra characters after close-brace")
	}

	for i := 0; i < len(patterns); i += 2 {
		pat := patterns[i]
		body := patterns[i+1]

		if pat == "default" || pat == val || matchGlob(pat, val) {
			return in.Eval(body)
		}
	}

	return "", nil
}

func builtinList(in *Interp, args []string) (string, error) {
	return strings.Join(args, " "), nil
}

func builtinLlength(in *Interp, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("wrong # args: should be \"llength list\"")
	}
	items := SplitTclList(args[0])
	return strconv.Itoa(len(items)), nil
}

func builtinLindex(in *Interp, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("wrong # args: should be \"lindex list index\"")
	}
	items := SplitTclList(args[0])
	idx, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("expected integer but got %q", args[1])
	}
	if idx < 0 || idx >= len(items) {
		return "", nil
	}
	return items[idx], nil
}

func builtinLappend(in *Interp, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("wrong # args: should be \"lappend varName ?value ...?\"")
	}
	varName := args[0]
	cur, _ := in.GetVar(varName)
	items := SplitTclList(cur)
	items = append(items, args[1:]...)
	res := strings.Join(items, " ")
	in.SetVar(varName, res)
	return res, nil
}

func builtinLrange(in *Interp, args []string) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("wrong # args: should be \"lrange list first last\"")
	}
	items := SplitTclList(args[0])
	first, err1 := strconv.Atoi(args[1])
	last, err2 := strconv.Atoi(args[2])
	if err1 != nil || err2 != nil {
		if args[2] == "end" {
			last = len(items) - 1
		} else {
			return "", fmt.Errorf("expected integer but got invalid range")
		}
	}
	if first < 0 {
		first = 0
	}
	if last >= len(items) {
		last = len(items) - 1
	}
	if first > last || first >= len(items) {
		return "", nil
	}
	return strings.Join(items[first:last+1], " "), nil
}

func builtinLinsert(in *Interp, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("wrong # args: should be \"linsert list index ?element ...?\"")
	}
	items := SplitTclList(args[0])
	idx, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("expected integer index: %w", err)
	}
	if idx < 0 {
		idx = 0
	}
	if idx > len(items) {
		idx = len(items)
	}

	elements := args[2:]
	var res []string
	res = append(res, items[:idx]...)
	res = append(res, elements...)
	res = append(res, items[idx:]...)
	return strings.Join(res, " "), nil
}

func builtinLreplace(in *Interp, args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("wrong # args: should be \"lreplace list first last ?element ...?\"")
	}
	items := SplitTclList(args[0])
	first, _ := strconv.Atoi(args[1])
	last, _ := strconv.Atoi(args[2])
	if first < 0 {
		first = 0
	}
	if last >= len(items) {
		last = len(items) - 1
	}

	var res []string
	if first < len(items) {
		res = append(res, items[:first]...)
	}
	res = append(res, args[3:]...)
	if last+1 < len(items) {
		res = append(res, items[last+1:]...)
	}
	return strings.Join(res, " "), nil
}

func builtinLsearch(in *Interp, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("wrong # args: should be \"lsearch list pattern\"")
	}
	items := SplitTclList(args[0])
	pat := args[1]
	for i, it := range items {
		if it == pat || matchGlob(pat, it) {
			return strconv.Itoa(i), nil
		}
	}
	return "-1", nil
}

func builtinJoin(in *Interp, args []string) (string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", fmt.Errorf("wrong # args: should be \"join list ?joinString?\"")
	}
	items := SplitTclList(args[0])
	joiner := " "
	if len(args) == 2 {
		joiner = args[1]
	}
	return strings.Join(items, joiner), nil
}

func builtinSplit(in *Interp, args []string) (string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", fmt.Errorf("wrong # args: should be \"split string ?splitChars?\"")
	}
	str := args[0]
	splitChars := " \t\n\r"
	if len(args) == 2 {
		splitChars = args[1]
	}
	if splitChars == "" {
		runes := []rune(str)
		var res []string
		for _, r := range runes {
			res = append(res, string(r))
		}
		return strings.Join(res, " "), nil
	}
	fields := strings.FieldsFunc(str, func(r rune) bool {
		return strings.ContainsRune(splitChars, r)
	})
	return strings.Join(fields, " "), nil
}

func builtinString(in *Interp, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("wrong # args: should be \"string subcommand ?argument ...?\"")
	}

	subcmd := args[0]
	switch subcmd {
	case "length":
		return strconv.Itoa(len(args[1])), nil
	case "index":
		if len(args) != 3 {
			return "", fmt.Errorf("wrong # args: should be \"string index string charIndex\"")
		}
		runes := []rune(args[1])
		idx, err := strconv.Atoi(args[2])
		if err != nil || idx < 0 || idx >= len(runes) {
			return "", nil
		}
		return string(runes[idx]), nil
	case "range":
		if len(args) != 4 {
			return "", fmt.Errorf("wrong # args: should be \"string range string first last\"")
		}
		runes := []rune(args[1])
		first, err1 := strconv.Atoi(args[2])
		last, err2 := strconv.Atoi(args[3])
		if err1 != nil || err2 != nil {
			if args[3] == "end" {
				last = len(runes) - 1
			} else {
				return "", nil
			}
		}
		if first < 0 {
			first = 0
		}
		if last >= len(runes) {
			last = len(runes) - 1
		}
		if first > last || first >= len(runes) {
			return "", nil
		}
		return string(runes[first : last+1]), nil
	case "tolower":
		return strings.ToLower(args[1]), nil
	case "toupper":
		return strings.ToUpper(args[1]), nil
	case "trim":
		chars := " \t\n\r"
		if len(args) == 3 {
			chars = args[2]
		}
		return strings.Trim(args[1], chars), nil
	case "trimleft":
		chars := " \t\n\r"
		if len(args) == 3 {
			chars = args[2]
		}
		return strings.TrimLeft(args[1], chars), nil
	case "trimright":
		chars := " \t\n\r"
		if len(args) == 3 {
			chars = args[2]
		}
		return strings.TrimRight(args[1], chars), nil
	case "compare":
		if len(args) != 3 {
			return "", fmt.Errorf("wrong # args: should be \"string compare string1 string2\"")
		}
		res := strings.Compare(args[1], args[2])
		return strconv.Itoa(res), nil
	case "equal":
		if len(args) != 3 {
			return "", fmt.Errorf("wrong # args: should be \"string equal string1 string2\"")
		}
		if args[1] == args[2] {
			return "1", nil
		}
		return "0", nil
	case "match":
		if len(args) != 3 {
			return "", fmt.Errorf("wrong # args: should be \"string match pattern string\"")
		}
		if matchGlob(args[1], args[2]) {
			return "1", nil
		}
		return "0", nil
	default:
		return "", fmt.Errorf("bad option %q: must be compare, equal, index, length, match, range, tolower, toupper, trim, trimleft, or trimright", subcmd)
	}
}

func matchGlob(pattern, s string) bool {
	matched, err := filepath.Match(pattern, s)
	return err == nil && matched
}

func builtinInfo(in *Interp, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("wrong # args: should be \"info subcommand ?argument ...?\"")
	}
	subcmd := args[0]
	switch subcmd {
	case "exists":
		if len(args) != 2 {
			return "", fmt.Errorf("wrong # args: should be \"info exists varName\"")
		}
		_, err := in.GetVar(args[1])
		if err == nil {
			return "1", nil
		}
		return "0", nil
	case "vars":
		in.mu.RLock()
		defer in.mu.RUnlock()
		var vars []string
		for k := range in.currentScope().vars {
			vars = append(vars, k)
		}
		for k := range in.globalScope().vars {
			vars = append(vars, k)
		}
		return strings.Join(vars, " "), nil
	case "coroutine":
		in.mu.RLock()
		name := in.curCoro
		in.mu.RUnlock()
		return name, nil
	case "procs":
		in.mu.RLock()
		defer in.mu.RUnlock()
		var procs []string
		for k := range in.procs {
			procs = append(procs, k)
		}
		return strings.Join(procs, " "), nil
	default:
		return "", fmt.Errorf("unknown info subcommand %q", subcmd)
	}
}

func builtinConcat(in *Interp, args []string) (string, error) {
	var words []string
	for _, a := range args {
		words = append(words, SplitTclList(a)...)
	}
	return strings.Join(words, " "), nil
}

func builtinFormat(in *Interp, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("wrong # args: should be \"format formatString ?arg ...?\"")
	}
	fmtStr := args[0]
	var fmtArgs []any
	for _, a := range args[1:] {
		if i, err := strconv.ParseInt(a, 10, 64); err == nil {
			fmtArgs = append(fmtArgs, i)
		} else if f, err := strconv.ParseFloat(a, 64); err == nil {
			fmtArgs = append(fmtArgs, f)
		} else {
			fmtArgs = append(fmtArgs, a)
		}
	}
	return fmt.Sprintf(fmtStr, fmtArgs...), nil
}

func builtinEval(in *Interp, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("wrong # args: should be \"eval arg ?arg ...?\"")
	}
	script := strings.Join(args, " ")
	return in.Eval(script)
}

func builtinGlobal(in *Interp, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("wrong # args: should be \"global varName ?varName ...?\"")
	}
	in.MarkGlobal(args...)
	return "", nil
}

func builtinBreak(in *Interp, args []string) (string, error) {
	return "", ErrBreak
}

func builtinContinue(in *Interp, args []string) (string, error) {
	return "", ErrContinue
}

// evalCondition evaluates a conditional expression string returning boolean true/false.
func (in *Interp) evalCondition(cond string) (bool, error) {
	cond = strings.TrimSpace(cond)
	if cond == "1" || strings.EqualFold(cond, "true") {
		return true, nil
	}
	if cond == "0" || strings.EqualFold(cond, "false") {
		return false, nil
	}

	res, err := in.evalExpr(cond)
	if err != nil {
		// Fallback: evaluate expression command
		res, err = in.Eval("expr " + cond)
		if err != nil {
			return false, err
		}
	}

	if num, err := strconv.ParseFloat(res, 64); err == nil {
		return num != 0, nil
	}
	if b, err := strconv.ParseBool(res); err == nil {
		return b, nil
	}
	return res != "" && res != "0", nil
}

// builtinExpr handles 'expr' expressions.
func builtinExpr(in *Interp, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("wrong # args: should be \"expr arg ?arg ...?\"")
	}
	exprStr := strings.Join(args, " ")
	return in.evalExpr(exprStr)
}

// evalExpr parses and computes infix arithmetic, comparison, and logical expressions.
func (in *Interp) evalExpr(exprStr string) (string, error) {
	// Substitute any variables or commands within the expression first
	subst, err := in.substituteString(exprStr)
	if err != nil {
		return "", err
	}

	tokens := strings.Fields(subst)
	if len(tokens) == 0 {
		return "0", nil
	}
	if len(tokens) == 1 {
		return tokens[0], nil
	}

	// Two operands: op1 + op2
	if len(tokens) == 3 {
		leftStr, op, rightStr := tokens[0], tokens[1], tokens[2]

		leftInt, errLInt := strconv.ParseInt(leftStr, 10, 64)
		rightInt, errRInt := strconv.ParseInt(rightStr, 10, 64)

		if errLInt == nil && errRInt == nil {
			switch op {
			case "+":
				return strconv.FormatInt(leftInt+rightInt, 10), nil
			case "-":
				return strconv.FormatInt(leftInt-rightInt, 10), nil
			case "*":
				return strconv.FormatInt(leftInt*rightInt, 10), nil
			case "/":
				if rightInt == 0 {
					return "", fmt.Errorf("divide by zero")
				}
				return strconv.FormatInt(leftInt/rightInt, 10), nil
			case "%":
				if rightInt == 0 {
					return "", fmt.Errorf("divide by zero")
				}
				return strconv.FormatInt(leftInt%rightInt, 10), nil
			case "==":
				if leftInt == rightInt {
					return "1", nil
				}
				return "0", nil
			case "!=":
				if leftInt != rightInt {
					return "1", nil
				}
				return "0", nil
			case "<":
				if leftInt < rightInt {
					return "1", nil
				}
				return "0", nil
			case "<=":
				if leftInt <= rightInt {
					return "1", nil
				}
				return "0", nil
			case ">":
				if leftInt > rightInt {
					return "1", nil
				}
				return "0", nil
			case ">=":
				if leftInt >= rightInt {
					return "1", nil
				}
				return "0", nil
			case "&&":
				if leftInt != 0 && rightInt != 0 {
					return "1", nil
				}
				return "0", nil
			case "||":
				if leftInt != 0 || rightInt != 0 {
					return "1", nil
				}
				return "0", nil
			case "**":
				res := math.Pow(float64(leftInt), float64(rightInt))
				return strconv.FormatInt(int64(res), 10), nil
			}
		}

		leftFloat, errLFloat := strconv.ParseFloat(leftStr, 64)
		rightFloat, errRFloat := strconv.ParseFloat(rightStr, 64)
		if errLFloat == nil && errRFloat == nil {
			switch op {
			case "+":
				return strconv.FormatFloat(leftFloat+rightFloat, 'g', -1, 64), nil
			case "-":
				return strconv.FormatFloat(leftFloat-rightFloat, 'g', -1, 64), nil
			case "*":
				return strconv.FormatFloat(leftFloat*rightFloat, 'g', -1, 64), nil
			case "/":
				if rightFloat == 0 {
					return "", fmt.Errorf("divide by zero")
				}
				return strconv.FormatFloat(leftFloat/rightFloat, 'g', -1, 64), nil
			case "==":
				if leftFloat == rightFloat {
					return "1", nil
				}
				return "0", nil
			case "!=":
				if leftFloat != rightFloat {
					return "1", nil
				}
				return "0", nil
			case "<":
				if leftFloat < rightFloat {
					return "1", nil
				}
				return "0", nil
			case "<=":
				if leftFloat <= rightFloat {
					return "1", nil
				}
				return "0", nil
			case ">":
				if leftFloat > rightFloat {
					return "1", nil
				}
				return "0", nil
			case ">=":
				if leftFloat >= rightFloat {
					return "1", nil
				}
				return "0", nil
			}
		}

		// String equality
		if op == "eq" || op == "==" {
			if leftStr == rightStr {
				return "1", nil
			}
			return "0", nil
		}
		if op == "ne" || op == "!=" {
			if leftStr != rightStr {
				return "1", nil
			}
			return "0", nil
		}
	}

	// Math functions: sqrt(x), abs(x), sin(x), cos(x), etc.
	if strings.Contains(subst, "(") && strings.HasSuffix(subst, ")") {
		openParen := strings.Index(subst, "(")
		fnName := strings.TrimSpace(subst[:openParen])
		argStr := strings.TrimSpace(subst[openParen+1 : len(subst)-1])
		argVal, err := in.evalExpr(argStr)
		if err == nil {
			f, err := strconv.ParseFloat(argVal, 64)
			if err == nil {
				switch fnName {
				case "abs":
					return strconv.FormatFloat(math.Abs(f), 'g', -1, 64), nil
				case "sqrt":
					return strconv.FormatFloat(math.Sqrt(f), 'g', -1, 64), nil
				case "sin":
					return strconv.FormatFloat(math.Sin(f), 'g', -1, 64), nil
				case "cos":
					return strconv.FormatFloat(math.Cos(f), 'g', -1, 64), nil
				case "tan":
					return strconv.FormatFloat(math.Tan(f), 'g', -1, 64), nil
				case "ceil":
					return strconv.FormatFloat(math.Ceil(f), 'g', -1, 64), nil
				case "floor":
					return strconv.FormatFloat(math.Floor(f), 'g', -1, 64), nil
				case "round":
					return strconv.FormatFloat(math.Round(f), 'g', -1, 64), nil
				}
			}
		}
	}

	return subst, nil
}
