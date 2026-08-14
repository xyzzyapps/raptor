package raptor

import (
	"context"
	"moarvm-go/engine"
	"moarvm-go/tcl"
	"os"
	"path/filepath"
	"testing"
)

func TestMoarVMDynamicLibraryCompilationAndLoading(t *testing.T) {
	// 1. Author a library in another language (Tcl)
	tclScript := `
# Tcl Math & Logic Library for MoarVM
proc compute_answer {} {
    set x 40
    incr x 2
    return $x
}

proc add_numbers {a b} {
    set res 100
    return $res
}
`

	// 2. Compile Tcl library into a MoarVM Dynamic Module (.moarvm container)
	compiler := tcl.NewCompiler()
	mod, err := compiler.CompileLibrary("tcl_math", tclScript)
	if err != nil {
		t.Fatalf("failed compiling Tcl library to MoarVM module: %v", err)
	}

	// Verify exported symbols
	if !mod.HasSymbol("compute_answer") {
		t.Fatalf("expected module to export 'compute_answer'")
	}
	if !mod.HasSymbol("add_numbers") {
		t.Fatalf("expected module to export 'add_numbers'")
	}

	// 3. Save module to temporary .moarvm dynamic library
	tempDir := t.TempDir()
	modPath := filepath.Join(tempDir, "tcl_math.moarvm")
	if err := mod.Save(modPath); err != nil {
		t.Fatalf("failed saving .moarvm module: %v", err)
	}

	// 4. Load the compiled MoarVM dynamic library in Raptor
	in := NewInterp()
	raptorCode := `
my $mod = moar_load_module("` + filepath.ToSlash(modPath) + `");
my $syms = $mod{"symbols"};
my $symCount = $syms.elems();
my $ans = moar_call_symbol($mod, "compute_answer");
$ans;
`
	val, err := in.Eval(raptorCode)
	if err != nil {
		t.Fatalf("Raptor failed executing MoarVM dynamic module: %v", err)
	}

	if val.IntVal <= 0 {
		t.Fatalf("expected positive integer result from MoarVM dynamic module, got %v", val)
	}
}

func TestMoarVMDynlibSymbolResolutionError(t *testing.T) {
	// Test error when symbol is not present in module
	mod := moargo.NewModule("test_mod", "tcl")
	mod.DefineProc("valid_symbol", 8)
	data, err := mod.Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	tempDir := t.TempDir()
	modPath := filepath.Join(tempDir, "test_mod.moarvm")
	if err := os.WriteFile(modPath, data, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	in := NewInterp()
	raptorCode := `
my $mod = moar_load_module("` + filepath.ToSlash(modPath) + `");
my $errRes = moar_call_symbol($mod, "missing_symbol");
`
	_, err = in.Eval(raptorCode)
	if err == nil {
		t.Fatalf("expected error for missing symbol, got nil")
	}
}

func TestMoarVMDynlibExecutionOnEngine(t *testing.T) {
	absPath := resolveTestMoarDLL()
	if absPath == "" {
		t.Skip("moar.dll not found, skipping engine test")
	}

	// Create and initialize MoarVM Engine
	vm, err := moargo.New(moargo.Config{
		DLLPath:  absPath,
		ProgName: "dynlib_engine_test",
	})
	if err != nil {
		t.Fatalf("failed creating MoarVM engine: %v", err)
	}
	ctx := context.Background()
	if err := vm.Init(ctx); err != nil {
		t.Fatalf("failed init MoarVM: %v", err)
	}
	defer vm.Destroy()

	// Compile and run dynamic module directly on MoarVM engine
	mod := moargo.NewModule("calc_mod", "tcl")
	frame, _ := mod.DefineProc("calc_main", 16)
	frame.EmitOp(moargo.OpConstI64)
	frame.EmitReg(0)
	frame.EmitInt64(100)
	frame.EmitOp(moargo.OpReturn)

	if err := mod.Execute(ctx, vm); err != nil {
		t.Fatalf("failed executing dynamic module on MoarVM: %v", err)
	}
}
