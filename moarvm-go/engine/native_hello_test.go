package moargo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func findMoarEXE(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "vendor", "MoarVM", "moar.exe"),
		filepath.Join("vendor", "MoarVM", "moar.exe"),
		filepath.Join("..", "..", "vendor", "MoarVM", "moar.exe"),
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	t.Skip("moar.exe not found")
	return ""
}

func TestNativeMoarSays42(t *testing.T) {
	moar := findMoarEXE(t)
	bc, err := EmitSayString("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(bc) < 96 || string(bc[:6]) != "MOARVM" {
		t.Fatalf("bad image: len=%d", len(bc))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.moarvm")
	if err := os.WriteFile(path, bc, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(moar, path)
	cmd.Dir = filepath.Dir(moar)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("moar.exe failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "42" {
		t.Fatalf("expected 42, got %q (raw %q)", got, out)
	}
}

func TestRunNativeDLLSays42(t *testing.T) {
	if FindMoarDLL() == "" {
		t.Skip("moar.dll not found")
	}
	bc, err := EmitSayString("42")
	if err != nil {
		t.Fatal(err)
	}
	out, err := RunNative(bc)
	if err != nil {
		t.Fatalf("RunNative: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "42" {
		t.Fatalf("expected 42, got %q", out)
	}
}
