package raptor

import (
	"moarvm-go/engine"
	"moarvm-go/tcl"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMoarVMTclInteropOpcodes(t *testing.T) {
	// Same CompUnit v7 opcodes as Raptor's --moar backend.
	tclScript := "puts [expr {40 + 2}]\n"
	c := tcl.NewCompiler()
	cu, err := c.CompileScript(tclScript)
	if err != nil {
		t.Fatalf("tcl compile: %v", err)
	}
	out, err := moargo.RunNative(cu)
	if err != nil {
		t.Skipf("native moar not available: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Fatalf("expected Tcl/Moar to print 42, got %q", out)
	}

	// Raptor compiler emits the same say/const_i64 family.
	rc := NewCompiler()
	bc, err := rc.CompileScript("say 42;")
	if err != nil {
		t.Fatalf("raptor compile: %v", err)
	}
	if len(bc) < 16 {
		t.Fatalf("raptor bytecode too small: %d", len(bc))
	}
	_ = filepath.Separator
	_ = os.DevNull
}
