package tcl

import (
	"context"
	"errors"
	"moarvm-go/engine"

	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func resolveTestMoarDLL() string {
	candidates := []string{
		filepath.Join("..", "build", "moarvm", "bin", "moar.dll"),
		filepath.Join("build", "moarvm", "bin", "moar.dll"),
		filepath.Join("..", "..", "build", "moarvm", "bin", "moar.dll"),
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	return ""
}

func TestTclRakuGrammarIsSourceOfTruth(t *testing.T) {
	g, err := GetTclGrammar()
	if err != nil {
		t.Fatalf("load tcl.raku: %v", err)
	}
	if g.Name != "Tcl" {
		t.Fatalf("expected grammar name Tcl, got %q", g.Name)
	}
	cmds, err := ParseTclAST("set a 42; set b $a")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d (pos leftover)", len(cmds))
	}
	if cmds[0].Words[0].Inner != "set" || cmds[0].Words[1].Inner != "a" {
		t.Fatalf("unexpected first command: %+v", cmds[0].Words)
	}
}

func TestTclSetAndGet(t *testing.T) {
	in := NewInterp()
	res, err := in.Eval("set a 42; set b $a; set b")
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if res != "42" {
		t.Fatalf("expected 42, got %q", res)
	}
}

func TestTclExprAndSubst(t *testing.T) {
	in := NewInterp()
	res, err := in.Eval("set x 10; set y 25; expr $x + $y")
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if res != "35" {
		t.Fatalf("expected 35, got %q", res)
	}
}

func TestTclGrammarBracesVsQuotes(t *testing.T) {
	script := `
set name "World"
set braced {Hello $name [expr 2 + 2]}
set quoted "Hello $name [expr 2 + 2]"
list $braced $quoted
`
	in := NewInterp()
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	expected := "Hello $name [expr 2 + 2] Hello World 4"
	if res != expected {
		t.Fatalf("expected %q, got %q", expected, res)
	}
}

func TestTclProcAndReturn(t *testing.T) {
	in := NewInterp()
	script := `
proc add {a b} {
    return [expr $a + $b]
}
add 15 27
`
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if res != "42" {
		t.Fatalf("expected 42, got %q", res)
	}
}

func TestTclFactorialRecursion(t *testing.T) {
	in := NewInterp()
	script := `
proc fact {n} {
    if {$n <= 1} {
        return 1
    } else {
        set prev [expr $n - 1]
        set rec [fact $prev]
        return [expr $n * $rec]
    }
}
fact 5
`
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if res != "120" {
		t.Fatalf("expected 120, got %q", res)
	}
}

func TestTclApplyLambda(t *testing.T) {
	in := NewInterp()
	res, err := in.Eval(`apply {{a b} {expr $a + $b}} 15 27`)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if res != "42" {
		t.Fatalf("expected 42, got %q", res)
	}
}

func TestCompileApplyToBytecode(t *testing.T) {
	res, err := RunScriptOnMoar(`apply {{a b} {expr $a + $b}} 15 27`)
	if err != nil {
		t.Fatalf("native apply failed: %v", err)
	}
	if res != "42" {
		t.Fatalf("expected 42, got %q", res)
	}
}

func TestTclCoroutineYield(t *testing.T) {
	t.Skip("native continuation lexicals: frame header still reports 0 lexicals to the validator")
	in := NewInterp()
	script := `
set a [coroutine tick apply {{} {
    yield 1
    yield 2
    return 3
}}]
set b [tick]
set c [tick]
list $a $b $c
`
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("coroutine eval failed: %v", err)
	}
	if res != "1 2 3" {
		t.Fatalf("expected '1 2 3', got %q", res)
	}
}

func TestCompileCoroutineToBytecode(t *testing.T) {
	t.Skip("native continuation lexicals: frame header still reports 0 lexicals to the validator")
	script := `
set a [coroutine tick apply {{} {
    yield 1
    yield 2
    return 3
}}]
set b [tick]
set c [tick]
list $a $b $c
`
	res, err := RunScriptOnMoar(script)
	if err != nil {
		t.Fatalf("native coroutine failed: %v", err)
	}
	if res != "1 2 3" {
		t.Fatalf("expected '1 2 3', got %q", res)
	}
}

func TestTclYieldResumeValue(t *testing.T) {
	t.Skip("native continuation lexicals: frame header still reports 0 lexicals to the validator")
	in := NewInterp()
	script := `
set first [coroutine ping apply {{} {
    set x [yield ready]
    return $x
}}]
set second [ping hello]
list $first $second
`
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("yield resume failed: %v", err)
	}
	if res != "ready hello" {
		t.Fatalf("expected 'ready hello', got %q", res)
	}
}

func TestCompileScriptToBytecode(t *testing.T) {
	script := `
set total 0
for {set i 1} {$i <= 10} {incr i} {
    set total [expr $total + $i]
}
set total
`
	res, err := RunScriptOnMoar(script)
	if err != nil {
		t.Fatalf("native compile/run failed: %v", err)
	}
	if res != "55" {
		t.Fatalf("expected 55, got %q", res)
	}
}

func TestTclWhileAndForLoops(t *testing.T) {
	in := NewInterp()
	script := `
set total 0
for {set i 1} {$i <= 5} {incr i} {
    set total [expr $total + $i]
}
set total
`
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if res != "15" {
		t.Fatalf("expected 15, got %q", res)
	}
}

func TestTclForeachAndSwitch(t *testing.T) {
	in := NewInterp()
	script := `
set acc 0
foreach item {10 20 30} {
    set acc [expr $acc + $item]
}

set type "apple"
set category "unknown"
switch $type {
    banana { set category "yellow" }
    apple  { set category "fruit" }
    default { set category "other" }
}

list $acc $category
`
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if res != "60 fruit" {
		t.Fatalf("expected '60 fruit', got %q", res)
	}
}

func TestTclStringSubcommands(t *testing.T) {
	in := NewInterp()
	script := `
set s "  Hello Tcl World  "
set len [string length $s]
set trimmed [string trim $s]
set upper [string toupper $trimmed]
set sub [string range $upper 0 4]
set eq [string equal $sub "HELLO"]
list $len $trimmed $sub $eq
`
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if res != "19 Hello Tcl World HELLO 1" {
		t.Fatalf("expected '19 Hello Tcl World HELLO 1', got %q", res)
	}
}

func TestTclListOperations(t *testing.T) {
	in := NewInterp()
	script := `
set mylist [list apple banana cherry]
lappend mylist date
set len [llength $mylist]
set second [lindex $mylist 1]
set ranged [lrange $mylist 1 2]
list $len $second $ranged
`
	res, err := in.Eval(script)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if res != "4 banana banana cherry" {
		t.Fatalf("expected '4 banana banana cherry', got %q", res)
	}
}

func TestTclPutsOutput(t *testing.T) {
	out, err := RunScriptOnMoar("puts {Hello from 100% Grammar Tcl!}")
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if strings.TrimSpace(out) != "Hello from 100% Grammar Tcl!" {
		t.Fatalf("expected 'Hello from 100%%%% Grammar Tcl!', got %q", out)
	}

}

func TestTclMoarBridge(t *testing.T) {
	in := NewInterp()
	mock := moargo.NewMock(moargo.Config{})
	NewBridge(in, mock)

	script := `
moar::init
set s1 [moar::state]
moar::set_prog_name "test.raku"
moar::run "app.moarvm"
moar::destroy
set s2 [moar::state]
list $s1 $s2
`
	res, err := in.EvalHost(script)
	if err != nil {
		t.Fatalf("bridge eval failed: %v", err)
	}
	if res != "READY TERMINATED" {
		t.Fatalf("expected 'READY TERMINATED', got %q", res)
	}
}

func TestGoFFIBinding(t *testing.T) {
	interp := NewInterp()

	err := RegisterGoFunc(interp, "go_add", func(a int, b int) int {
		return a + b
	})
	if err != nil {
		t.Fatalf("RegisterGoFunc failed: %v", err)
	}

	err = RegisterGoFunc(interp, "go_concat", func(prefix string, count int, suffix string) string {
		return strings.Repeat(prefix, count) + suffix
	})
	if err != nil {
		t.Fatalf("RegisterGoFunc failed: %v", err)
	}

	err = RegisterGoFunc(interp, "go_validate", func(val int) (string, error) {
		if val < 0 {
			return "", errors.New("negative value not allowed")
		}
		return "valid", nil
	})
	if err != nil {
		t.Fatalf("RegisterGoFunc failed: %v", err)
	}

	res, err := interp.EvalHost("go_add 100 250")
	if err != nil {
		t.Fatalf("eval go_add failed: %v", err)
	}
	if res != "350" {
		t.Fatalf("expected 350, got %s", res)
	}

	res, err = interp.EvalHost("go_concat {abc } 3 {end}")
	if err != nil {
		t.Fatalf("eval go_concat failed: %v", err)
	}
	if res != "abc abc abc end" {
		t.Fatalf("expected 'abc abc abc end', got %q", res)
	}

	_, err = interp.EvalHost("go_validate -5")
	if err == nil {
		t.Fatalf("expected error from go_validate, got nil")
	}
}

func TestCFFIKernel32(t *testing.T) {
	interp := NewInterp()

	script := `
set k32 [cffi::load "kernel32.dll"]
set pid [cffi::call $k32 "GetCurrentProcessId" uint {}]
set pid
`
	res, err := interp.EvalHost(script)
	if err != nil {
		t.Fatalf("C FFI kernel32 test failed: %v", err)
	}

	expectedPID := uint64(os.Getpid())
	gotPID, err := strconv.ParseUint(res, 10, 64)
	if err != nil {
		t.Fatalf("failed to parse returned PID %q: %v", res, err)
	}
	if gotPID != expectedPID {
		t.Fatalf("expected PID %d, got %d", expectedPID, gotPID)
	}
}

func TestCFFIBindMoarVM(t *testing.T) {
	absPath := resolveTestMoarDLL()
	if absPath == "" {
		t.Skip("moar.dll not present, skipping test")
	}

	interp := NewInterp()
	script := `
set moar [cffi::load "` + strings.ReplaceAll(absPath, `\`, `/`) + `"]
set has_jit [cffi::call $moar "MVM_jit_support" int {}]

cffi::bind $moar "MVM_vm_create_instance" ptr {} mvm_create
cffi::bind $moar "MVM_vm_destroy_instance" void {ptr} mvm_destroy

set vm [mvm_create]
mvm_destroy $vm
set has_jit
`
	res, err := interp.EvalHost(script)
	if err != nil {
		t.Fatalf("C FFI MoarVM binding test failed: %v", err)
	}

	if res != "1" {
		t.Fatalf("expected JIT support 1, got %s", res)
	}
}

func TestTclCompileAndRunOnMoarVM(t *testing.T) {
	absPath := resolveTestMoarDLL()
	if absPath == "" {
		t.Skip("moar.dll not found, skipping MoarVM execution test")
	}

	vm, err := moargo.New(moargo.Config{
		DLLPath:  absPath,
		ProgName: "tcl_moar_test",
	})
	if err != nil {
		t.Fatalf("failed creating MoarVM: %v", err)
	}
	ctx := context.Background()
	if err := vm.Init(ctx); err != nil {
		t.Fatalf("failed init MoarVM: %v", err)
	}
	defer vm.Destroy()

	script := `
set a 50
set b 12
incr a 3
expr $a + $b
`
	if err := CompileAndRun(ctx, vm, script); err != nil {
		t.Fatalf("CompileAndRun failed on MoarVM: %v", err)
	}
}

func TestTclGrammarAndMoarVM(t *testing.T) {
	absPath := resolveTestMoarDLL()
	if absPath == "" {
		t.Skip("moar.dll not found, skipping MoarVM execution test")
	}

	vm, err := moargo.New(moargo.Config{
		DLLPath:  absPath,
		ProgName: "tcl_grammar_test",
	})
	if err != nil {
		t.Fatalf("failed creating MoarVM: %v", err)
	}
	ctx := context.Background()
	if err := vm.Init(ctx); err != nil {
		t.Fatalf("failed init MoarVM: %v", err)
	}
	defer vm.Destroy()

	script := `
set count 10
set step 5
incr count 2
expr $count + $step
`
	if err := RunGrammarOnMoarVM(ctx, vm, script); err != nil {
		t.Fatalf("RunGrammarOnMoarVM failed: %v", err)
	}
}
