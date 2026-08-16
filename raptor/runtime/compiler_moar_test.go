package raptor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestMoarCompileSay42(t *testing.T) {
	c := NewCompiler()
	bc, err := c.CompileScript("say 42;")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(bc) < 16 {
		t.Fatalf("bytecode too small: %d", len(bc))
	}
}

func TestMoarCompileRejectsUnsupported(t *testing.T) {
	c := NewCompiler()
	_, err := c.CompileScript("my @a = [1, 2, 3]; say @a[0];")
	if err == nil {
		t.Fatal("expected error for array literal")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("want unsupported, got %v", err)
	}
}

func TestMoarNoGoFallback(t *testing.T) {
	in := NewInterp()
	var buf strings.Builder
	in.SetStdout(&buf)
	_, err := in.EvalOnBackend("use This::Does::Not::Exist;", BackendMoar)
	if err == nil {
		t.Fatal("expected moar compile error, not silent go run")
	}
	if !strings.Contains(err.Error(), "moar") && !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("want moar error, got %v", err)
	}
}

func TestMoarGoParitySubset(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "bin", "raptor.exe")
	if !fileExists(exe) {
		exe = filepath.Join(root, "bin", "raptor")
	}
	if !fileExists(exe) {
		t.Skip("raptor binary not built")
	}
	src := `say 40 + 2;
say "hi";
if 0 { say "THEN" } else { say "ELSE" }
if 1 { say "THEN" } else { say "ELSE" }
my $i = 0;
while $i < 3 { $i += 1; }
say $i;
say 2 ** 10;
say 0x0F +| 0xF0;
say "A" x 5;
say 10 min 20;
say 10 max 20;
my $v = Nil // "def";
say $v;
my $a = "keep";
$a //= "no";
say $a;
say 10 > 5 ?? "yes" !! "no";
`
	dir := t.TempDir()
	path := filepath.Join(dir, "parity.rp")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	goOut, err := runRaptor(exe, "--go", path)
	if err != nil {
		t.Fatalf("go: %v\n%s", err, goOut)
	}
	moarOut, err := runRaptor(exe, "--moar", path)
	if err != nil {
		t.Fatalf("moar: %v\n%s", err, moarOut)
	}
	norm := func(s string) string {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
		return strings.TrimSpace(s)
	}
	if norm(goOut) != norm(moarOut) {
		t.Fatalf("output mismatch\n go (%q):\n%s\n moar (%q):\n%s", goOut, goOut, moarOut, moarOut)
	}
}

func runRaptor(exe string, args ...string) (string, error) {
	cmd := exec.Command(exe, args...)
	if abs, err := filepath.Abs(".."); err == nil {
		cmd.Dir = abs
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
