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

func TestMoarCompileArray(t *testing.T) {
	c := NewCompiler()
	bc, err := c.CompileScript("my @a = [1, 2, 3]; say @a[0];")
	if err != nil {
		t.Fatalf("array compile: %v", err)
	}
	if len(bc) < 16 {
		t.Fatalf("bytecode too small: %d", len(bc))
	}
}

func TestMoarCompileRejectsUnsupported(t *testing.T) {
	c := NewCompiler()
	_, err := c.CompileScript("use This::Does::Not::Exist;")
	if err == nil {
		t.Fatal("expected error for missing module")
	}
	if !strings.Contains(err.Error(), "moar") && !strings.Contains(err.Error(), "cannot find") {
		t.Fatalf("want moar module error, got %v", err)
	}
}

func TestMoarCompileLanguageSurface(t *testing.T) {
	srcs := []string{
		`my @a = [1, 2] xx 2; is_deeply(@a, [1, 2, 1, 2], "xx");`,
		`my %h = { "a" => 1 }; say exists(%h, "a"); delete(%h, "a");`,
		`given 7 { when 7 { say "ok" } default { say "no" } }`,
		`enum Color <Red Green Blue>; say Red;`,
		`sub fib($n) { if $n <= 1 { return $n; } return fib($n - 1) + fib($n - 2); } say fib(6);`,
		`my $fn = sub ($x) { return $x + 1; }; say $fn(3);`,
		`subset Positive where { $_ > 0 }; my Positive $n = 3;`,
		`multi sub classify($n where { $n <= 1 }) { return $n; } multi sub classify($n) { return $n + 1; }`,
		`struct Point { int32 $x; int32 $y; } my $p = Point.new(); $p.x = 1;`,
		`if 25 == any(10, 20, 25) { say "yes"; }`,
		`my @g = gather { take 1; take 2; };`,
		`ok("hello" =~ "ell", "re");`,
		`package Foo { sub bar($x) { return $x; } } say Foo::bar(1);`,
		`goto L; say "no"; L: say "yes";`,
		`say substr("abcdef", 1, 3); say index("abc", "b");`,
		`my $s = join("-", [1, 2, 3]);`,
		`like("hello", "ell", "like");`,
		`say 3 <=> 1; say "b" cmp "a";`,
		`$_ = 1; say $_ for 1..2;`,
		`my $n = 0; my $h = 0; while $n < 2 { $h = $h + 1; $n = $n + 1; redo if $h == 1; } say $h;`,
	}
	for _, src := range srcs {
		c := NewCompiler()
		if _, err := c.CompileScript(src); err != nil {
			t.Errorf("compile %q: %v", src, err)
		}
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
my @xs = [1, 2, 3];
say @xs[0];
say elems(@xs);
my %h = { "k" => "v" };
say %h{"k"};
sub add($x, $y) { return $x + $y; }
say add(20, 22);
for 1..3 { say $_; }
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
