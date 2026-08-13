package moargo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMockEngineLifecycle(t *testing.T) {
	ctx := context.Background()
	engine := NewMock(Config{
		ProgName: "test.raku",
		Args:     []string{"arg1", "arg2"},
	})

	if engine.State() != StateUninitialized {
		t.Fatalf("expected state UNINITIALIZED, got %s", engine.State())
	}

	if err := engine.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if engine.State() != StateReady {
		t.Fatalf("expected state READY, got %s", engine.State())
	}

	if err := engine.RunFile(ctx, "dummy.moarvm"); err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}

	if err := engine.Destroy(); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	if engine.State() != StateTerminated {
		t.Fatalf("expected state TERMINATED, got %s", engine.State())
	}
}

func TestNativeVMLifecycle(t *testing.T) {
	dllPath := filepath.Join("..", "build", "moarvm", "bin", "moar.dll")
	absPath, err := filepath.Abs(dllPath)
	if err != nil {
		t.Fatalf("failed to resolve dll path: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skip("moar.dll not found, skipping native test")
	}

	ctx := context.Background()
	vm, err := New(Config{
		DLLPath:  absPath,
		ProgName: "native_test",
		Args:     []string{"--test"},
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if err := vm.Init(ctx); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if vm.State() != StateReady {
		t.Fatalf("expected state READY, got %s", vm.State())
	}

	if err := vm.Destroy(); err != nil {
		t.Fatalf("Destroy() failed: %v", err)
	}

	if vm.State() != StateTerminated {
		t.Fatalf("expected state TERMINATED, got %s", vm.State())
	}
}

func TestBytecodeEmitterAndExecution(t *testing.T) {
	dllPath := filepath.Join("..", "build", "moarvm", "bin", "moar.dll")
	absPath, err := filepath.Abs(dllPath)
	if err != nil {
		t.Fatalf("failed to resolve dll path: %v", err)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skip("moar.dll not found, skipping bytecode execution test")
	}

	// 1. Build a MoarVM Compilation Unit (.moarvm)
	cu := NewCompUnitEmitter("test_hll")
	f := cu.NewFrame("main", 3)
	f.SetLocalType(0, RegInt64)
	f.SetLocalType(1, RegInt64)
	f.SetLocalType(2, RegInt64)

	// Emit bytecode:
	// const_i64 reg0, 40
	f.EmitOp(OpConstI64)
	f.EmitReg(0)
	f.EmitInt64(40)

	// const_i64 reg1, 2
	f.EmitOp(OpConstI64)
	f.EmitReg(1)
	f.EmitInt64(2)

	// add_i reg2, reg0, reg1
	f.EmitOp(OpAddI)
	f.EmitReg(2)
	f.EmitReg(0)
	f.EmitReg(1)

	// return
	f.EmitOp(OpReturn)

	bc, err := cu.Emit()
	if err != nil {
		t.Fatalf("failed to emit bytecode: %v", err)
	}

	if len(bc) < 92 {
		t.Fatalf("bytecode too short: %d bytes", len(bc))
	}

	// 2. Execute on native MoarVM
	ctx := context.Background()
	vm, err := New(Config{
		DLLPath:  absPath,
		ProgName: "bytecode_test",
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if err := vm.Init(ctx); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer vm.Destroy()

	if err := vm.RunBytecode(ctx, bc); err != nil {
		t.Fatalf("RunBytecode failed on native MoarVM: %v", err)
	}
}
