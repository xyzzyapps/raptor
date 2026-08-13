package raptor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlurpAndSpurt(t *testing.T) {
	in := NewInterp()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_io.txt")

	code := `
my $path = "` + filepath.ToSlash(testFile) + `";
spurt($path, "Hello MoarVM IO\nLine 2\n");
my $content = slurp($path);
$content;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	expected := "Hello MoarVM IO\nLine 2\n"
	if val.StrVal != expected {
		t.Fatalf("expected %q, got %q", expected, val.StrVal)
	}
}

func TestFileHandleOpenReadlineClose(t *testing.T) {
	in := NewInterp()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_lines.txt")
	err := os.WriteFile(testFile, []byte("First Line\r\nSecond Line\r\n"), 0644)
	if err != nil {
		t.Fatalf("failed creating test file: %v", err)
	}

	code := `
my $path = "` + filepath.ToSlash(testFile) + `";
my $fh = open($path, "r");
my $line1 = readline($fh);
my $line2 = readline($fh);
close($fh);
[$line1, $line2];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected array of 2 lines, got %v", val)
	}
	if val.ArrayVal[0].StrVal != "First Line" || val.ArrayVal[1].StrVal != "Second Line" {
		t.Fatalf("lines mismatch: got [%q, %q]", val.ArrayVal[0].StrVal, val.ArrayVal[1].StrVal)
	}
}

func TestPerlStringAndHashBuiltins(t *testing.T) {
	in := NewInterp()
	code := `
my $s = "Hello World\n";
my $c = chomp($s);
my $l = length($c);
my $idx = index($c, "World");
my $fmt = sprintf("Count: %04d, Item: %s", 42, "MoarVM");

my %h = { "name" => "Raku5", "ver" => 5 };
my $ex = exists(%h, "name");
my $del = delete(%h, "ver");
my $nex = exists(%h, "ver");

[$c, $l, $idx, $fmt, $ex, $del, $nex];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 7 {
		t.Fatalf("expected array of 7, got %v", val)
	}

	if val.ArrayVal[0].StrVal != "Hello World" {
		t.Errorf("chomp: expected 'Hello World', got %q", val.ArrayVal[0].StrVal)
	}
	if val.ArrayVal[1].IntVal != 11 {
		t.Errorf("length: expected 11, got %d", val.ArrayVal[1].IntVal)
	}
	if val.ArrayVal[2].IntVal != 6 {
		t.Errorf("index: expected 6, got %d", val.ArrayVal[2].IntVal)
	}
	if val.ArrayVal[3].StrVal != "Count: 0042, Item: MoarVM" {
		t.Errorf("sprintf: expected 'Count: 0042, Item: MoarVM', got %q", val.ArrayVal[3].StrVal)
	}
	if !val.ArrayVal[4].BoolVal {
		t.Errorf("exists name: expected true")
	}
	if val.ArrayVal[5].IntVal != 5 {
		t.Errorf("delete ver: expected 5, got %v", val.ArrayVal[5])
	}
	if val.ArrayVal[6].BoolVal {
		t.Errorf("exists ver after delete: expected false")
	}
}
