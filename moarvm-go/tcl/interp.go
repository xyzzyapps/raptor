package tcl

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"unicode"
)

// Interp is the core Tcl interpreter environment.
type Interp struct {
	mu       sync.RWMutex
	commands map[string]CommandFunc
	procs    map[string]*Proc
	scopes   []*Scope
	stdout   io.Writer
	logger   *slog.Logger
	cffiMgr  *CFFIManager
}

// NewInterp creates a new initialized Tcl interpreter with all standard built-in commands and C/Go FFI.
func NewInterp() *Interp {
	interp := &Interp{
		commands: make(map[string]CommandFunc),
		procs:    make(map[string]*Proc),
		scopes:   []*Scope{NewScope()},
		stdout:   os.Stdout,
		logger:   slog.Default(),
	}

	interp.registerBuiltins()
	interp.cffiMgr = NewCFFIManager(interp.logger)
	interp.cffiMgr.Register(interp)
	RegisterGoFFI(interp)

	return interp
}

// SetLogger configures the structured logger for the interpreter.
func (in *Interp) SetLogger(logger *slog.Logger) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if logger != nil {
		in.logger = logger
	}
}

// SetStdout configures the standard output writer for commands like 'puts'.
func (in *Interp) SetStdout(w io.Writer) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if w != nil {
		in.stdout = w
	}
}

// RegisterCommand adds or overrides a command in the interpreter.
func (in *Interp) RegisterCommand(name string, fn CommandFunc) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.commands[name] = fn
	in.logger.Debug("registered tcl command", slog.String("name", name))
}

// PushScope pushes a new local scope onto the call stack.
func (in *Interp) PushScope() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.scopes = append(in.scopes, NewScope())
}

// PopScope pops the top local scope from the call stack.
func (in *Interp) PopScope() {
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.scopes) > 1 {
		in.scopes = in.scopes[:len(in.scopes)-1]
	}
}

// currentScope returns the active variable scope.
func (in *Interp) currentScope() *Scope {
	return in.scopes[len(in.scopes)-1]
}

// globalScope returns the top-level global variable scope.
func (in *Interp) globalScope() *Scope {
	return in.scopes[0]
}

// SetVar sets a variable value in the active scope (or global if marked global).
func (in *Interp) SetVar(name string, val string) {
	in.mu.Lock()
	defer in.mu.Unlock()

	scope := in.currentScope()
	if scope.globals[name] || len(in.scopes) == 1 {
		in.globalScope().vars[name] = val
	} else {
		scope.vars[name] = val
	}
	in.logger.Debug("set tcl variable", slog.String("name", name), slog.String("val", val))
}

// GetVar retrieves a variable value from the current or global scope.
func (in *Interp) GetVar(name string) (string, error) {
	in.mu.RLock()
	defer in.mu.RUnlock()

	scope := in.currentScope()
	if scope.globals[name] || len(in.scopes) == 1 {
		if val, ok := in.globalScope().vars[name]; ok {
			return val, nil
		}
		return "", fmt.Errorf("%w: %s", ErrVarNotFound, name)
	}

	if val, ok := scope.vars[name]; ok {
		return val, nil
	}
	return "", fmt.Errorf("%w: %s", ErrVarNotFound, name)
}

// MarkGlobal declares variable names as global in the current local scope.
func (in *Interp) MarkGlobal(names ...string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	scope := in.currentScope()
	for _, name := range names {
		scope.globals[name] = true
	}
}

// Eval parses and executes a Tcl script 100% through the Grammar engine.
func (in *Interp) Eval(script string) (string, error) {
	return ParseTclWithGrammar(in, script)
}

// resolveWord applies strict standard Tcl substitution rules to a parsed word token.
func (in *Interp) resolveWord(str, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 {
		return "", nil
	}

	// 1. Braced string {raw content} -> No substitutions performed
	if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}") && len(raw) >= 2 {
		return str, nil
	}

	// 2. Quoted string "..." -> Substitutions performed, quotes stripped
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") && len(raw) >= 2 {
		return in.substituteString(str)
	}

	// 3. Command substitution [...]
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") && len(raw) >= 2 {
		return in.Eval(raw[1 : len(raw)-1])
	}

	// 4. Bare word or mixed substitutions
	return in.substituteString(raw)
}

// substituteString performs variable ($var), command ([...]), and backslash (\n, etc.) substitutions.
func (in *Interp) substituteString(s string) (string, error) {
	var sb strings.Builder
	runes := []rune(s)
	pos := 0

	for pos < len(runes) {
		ch := runes[pos]

		// Variable substitution: $var, ${var}, $arr(idx)
		if ch == '$' {
			pos++
			if pos >= len(runes) {
				sb.WriteRune('$')
				break
			}

			if runes[pos] == '{' {
				pos++
				var varNameSb strings.Builder
				for pos < len(runes) && runes[pos] != '}' {
					varNameSb.WriteRune(runes[pos])
					pos++
				}
				if pos < len(runes) && runes[pos] == '}' {
					pos++
				}
				val, err := in.GetVar(varNameSb.String())
				if err != nil {
					return "", err
				}
				sb.WriteString(val)
				continue
			}

			var varNameSb strings.Builder
			for pos < len(runes) && (unicode.IsLetter(runes[pos]) || unicode.IsDigit(runes[pos]) || runes[pos] == '_' || runes[pos] == ':') {
				varNameSb.WriteRune(runes[pos])
				pos++
			}
			varName := varNameSb.String()
			if len(varName) == 0 {
				sb.WriteRune('$')
				continue
			}

			val, err := in.GetVar(varName)
			if err != nil {
				return "", err
			}
			sb.WriteString(val)
			continue
		}

		// Command substitution: [...]
		if ch == '[' {
			pos++
			depth := 1
			var cmdSb strings.Builder
			for pos < len(runes) && depth > 0 {
				c := runes[pos]
				if c == '[' {
					depth++
					cmdSb.WriteRune(c)
					pos++
				} else if c == ']' {
					depth--
					if depth == 0 {
						pos++
						break
					}
					cmdSb.WriteRune(c)
					pos++
				} else if c == '\\' && pos+1 < len(runes) {
					cmdSb.WriteRune(c)
					pos++
					cmdSb.WriteRune(runes[pos])
					pos++
				} else {
					cmdSb.WriteRune(c)
					pos++
				}
			}

			res, err := in.Eval(cmdSb.String())
			if err != nil {
				return "", err
			}
			sb.WriteString(res)
			continue
		}

		// Backslash escape sequence: \n, \t, \r, \", \$, \[, \{, \\
		if ch == '\\' {
			pos++
			if pos >= len(runes) {
				sb.WriteRune('\\')
				break
			}
			esc := runes[pos]
			pos++
			switch esc {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case 'a':
				sb.WriteRune('\a')
			case 'b':
				sb.WriteRune('\b')
			case 'f':
				sb.WriteRune('\f')
			case 'v':
				sb.WriteRune('\v')
			default:
				sb.WriteRune(esc)
			}
			continue
		}

		sb.WriteRune(ch)
		pos++
	}

	return sb.String(), nil
}

// EvalWords invokes a command with parsed and substituted argument words.
func (in *Interp) EvalWords(words []string) (string, error) {
	if len(words) == 0 {
		return "", nil
	}

	cmdName := words[0]
	args := words[1:]

	in.mu.RLock()
	cmd, ok := in.commands[cmdName]
	proc, isProc := in.procs[cmdName]
	in.mu.RUnlock()

	if ok {
		return cmd(in, args)
	}

	if isProc {
		return in.invokeProc(proc, args)
	}

	return "", fmt.Errorf("%w: \"%s\"", ErrCommandNotFound, cmdName)
}

func (in *Interp) invokeProc(proc *Proc, args []string) (string, error) {
	if len(args) != len(proc.Args) {
		return "", fmt.Errorf("wrong # args: should be \"%s %s\"", proc.Name, strings.Join(proc.Args, " "))
	}

	in.PushScope()
	defer in.PopScope()

	for i, argName := range proc.Args {
		in.SetVar(argName, args[i])
	}

	res, err := in.Eval(proc.Body)
	if err != nil {
		if sig, ok := err.(*ReturnSignal); ok {
			return sig.Value, nil
		}
		return "", err
	}
	return res, nil
}
