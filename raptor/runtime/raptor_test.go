package raptor

import (
	"context"
	"moarvm-go/engine"
	"os"
	"path/filepath"
	"strings"
	"testing"
)


func TestScopingAndAutoload(t *testing.T) {
	in := NewInterp()
	code := `
my $x = 10;
if True {
    my $x = 99;
}
`
	_, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
}

func TestRaku5TypedVariables(t *testing.T) {
	in := NewInterp()
	code := `
my Int $x = 42;
my Str $msg = "hello";
my Num $pi = 3.14;
$msg ~ " " ~ $x ~ " " ~ $pi;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "hello 42 3.14" {
		t.Fatalf("expected 'hello 42 3.14', got %q", val.StrVal)
	}
}

func TestRaku5TypeMismatch(t *testing.T) {
	in := NewInterp()
	code := `
subset Integer where { $_ ~~ "Int" };
my Integer $x = "not an integer";
`
	_, err := in.Eval(code)
	if err == nil {
		t.Fatalf("expected type check failure, but got success")
	}
	if !strings.Contains(err.Error(), "type check failed") {
		t.Fatalf("expected 'type check failed', got %v", err)
	}
}

func TestRaku5MultipleDispatchByType(t *testing.T) {
	in := NewInterp()
	code := `
multi sub describe(Int $x) {
    return "int:" ~ $x;
}

multi sub describe(Str $s) {
    return "str:" ~ $s;
}

my $r1 = describe(100);
my $r2 = describe("raku5");
$r1 ~ " | " ~ $r2;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "int:100 | str:raku5" {
		t.Fatalf("expected 'int:100 | str:raku5', got %q", val.StrVal)
	}
}

func TestRaku5MultipleDispatchByArity(t *testing.T) {
	in := NewInterp()
	code := `
multi sub calc(Int $a, Int $b) {
    return $a + $b;
}

multi sub calc(Int $a, Int $b, Int $c) {
    return $a * $b + $c;
}

my $v1 = calc(10, 20);
my $v2 = calc(2, 3, 4);
$v1 ~ " " ~ $v2;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "30 10" {
		t.Fatalf("expected '30 10', got %q", val.StrVal)
	}
}

func TestRaku5MultipleDispatchSpecificity(t *testing.T) {
	in := NewInterp()
	code := `
multi sub handle(Any $x) {
    return "fallback_any";
}

multi sub handle(Int $x) {
    return "specific_int";
}

my $res1 = handle(99);
my $res2 = handle("text");
$res1 ~ " & " ~ $res2;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.StrVal != "specific_int & fallback_any" {
		t.Fatalf("expected 'specific_int & fallback_any', got %q", val.StrVal)
	}
}

func resolveTestMoarDLL() string {
	candidates := []string{
		filepath.Join("..", "bin", "moar.dll"),
		filepath.Join("bin", "moar.dll"),
		filepath.Join("..", "..", "moarvm-go", "build", "moarvm", "bin", "moar.dll"),
		filepath.Join("..", "build", "moarvm", "bin", "moar.dll"),
		filepath.Join("build", "moarvm", "bin", "moar.dll"),
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

func TestRaku5CompileAndRunOnMoarVM(t *testing.T) {
	absPath := resolveTestMoarDLL()
	if absPath == "" {
		t.Skip("moar.dll not found, skipping MoarVM test")
	}

	vm, err := moargo.New(moargo.Config{
		DLLPath:  absPath,
		ProgName: "raku5_moar_test",
	})
	if err != nil {
		t.Fatalf("failed creating MoarVM: %v", err)
	}
	ctx := context.Background()
	if err := vm.Init(ctx); err != nil {
		t.Fatalf("failed init MoarVM: %v", err)
	}
	defer vm.Destroy()

	code := `
my Int $x = 25;
my Int $y = 75;
my Int $sum = $x + $y;
`
	if err := CompileAndRun(ctx, vm, code); err != nil {
		t.Fatalf("CompileAndRun on MoarVM failed: %v", err)
	}
}

func TestMoarVMControlFlowAndFunctions(t *testing.T) {
	absPath := resolveTestMoarDLL()
	if absPath == "" {
		t.Skip("moar.dll not found, skipping MoarVM test")
	}

	vm, err := moargo.New(moargo.Config{
		DLLPath:  absPath,
		ProgName: "raku5_moar_control_test",
	})
	if err != nil {
		t.Fatalf("failed creating MoarVM: %v", err)
	}
	ctx := context.Background()
	if err := vm.Init(ctx); err != nil {
		t.Fatalf("failed init MoarVM: %v", err)
	}
	defer vm.Destroy()

	code := `
my Int $a = 15;
my Int $b = 30;
my Int $c = $a * 2 + $b - 5;
`
	if err := CompileAndRun(ctx, vm, code); err != nil {
		t.Fatalf("CompileAndRun computation on MoarVM failed: %v", err)
	}
}

func TestRaku5Perl5Bridge(t *testing.T) {
	in := NewInterp()
	code := `
my $res = eval_perl5("15 * 3");
$res;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Skipf("perl not found in system: %v", err)
	}
	if val.IntVal != 45 && val.StrVal != "45" {
		t.Fatalf("expected 45, got %s", val.String())
	}
}


