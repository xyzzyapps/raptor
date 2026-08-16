package moargo

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindMoarDLL locates moar.dll / libmoar for in-process embedding.
// Search order: directory of this process, then common repo layout paths.
func FindMoarDLL() string {
	if p := os.Getenv("MOAR_DLL"); p != "" {
		if abs, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	var names []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		names = append(names,
			filepath.Join(dir, "moar.dll"),
			filepath.Join(dir, "libmoar.dll"),
			filepath.Join(dir, "libmoar.so"),
			filepath.Join(dir, "libmoar.dylib"),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		names = append(names,
			filepath.Join(wd, "moar.dll"),
			filepath.Join(wd, "bin", "moar.dll"),
			filepath.Join(wd, "vendor", "MoarVM", "moar.dll"),
			filepath.Join(wd, "build", "moarvm", "bin", "moar.dll"),
			filepath.Join(wd, "..", "bin", "moar.dll"),
			filepath.Join(wd, "..", "vendor", "MoarVM", "moar.dll"),
		)
	}
	names = append(names,
		filepath.Join("bin", "moar.dll"),
		filepath.Join("vendor", "MoarVM", "moar.dll"),
		filepath.Join("build", "moarvm", "bin", "moar.dll"),
		filepath.Join("..", "bin", "moar.dll"),
		filepath.Join("..", "vendor", "MoarVM", "moar.dll"),
		filepath.Join("..", "..", "vendor", "MoarVM", "moar.dll"),
	)
	for _, c := range names {
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	return ""
}

// FindMoarEXE locates a built moar executable (optional; DLL is preferred).
func FindMoarEXE() string {
	candidates := []string{
		filepath.Join("vendor", "MoarVM", "moar.exe"),
		filepath.Join("..", "vendor", "MoarVM", "moar.exe"),
		filepath.Join("..", "..", "vendor", "MoarVM", "moar.exe"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{
			filepath.Join(filepath.Dir(exe), "moar.exe"),
			filepath.Join(filepath.Dir(exe), "moar"),
		}, candidates...)
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

// RunNative executes CompUnit bytes on the embedded MoarVM library and
// returns captured stdout (for Eval). Prefers moar.dll; falls back to moar.exe.
func RunNative(bytecode []byte) (stdout string, err error) {
	if dll := FindMoarDLL(); dll != "" {
		return runOnDLL(dll, bytecode, true)
	}
	return RunMoarEXE(bytecode)
}

// ExecNative runs bytecode in-process and leaves say/print on the process
// stdout — use this in a shipped CLI so the user sees output directly.
func ExecNative(bytecode []byte) error {
	if dll := FindMoarDLL(); dll != "" {
		_, err := runOnDLL(dll, bytecode, false)
		return err
	}
	out, err := RunMoarEXE(bytecode)
	if err != nil {
		return err
	}
	if out != "" {
		fmt.Println(out)
	}
	return nil
}

func runOnDLL(dllPath string, bytecode []byte, capture bool) (string, error) {
	vm, err := New(Config{
		DLLPath:  dllPath,
		ProgName: "tcl",
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	run := func() error {
		if err := vm.Init(ctx); err != nil {
			return err
		}
		defer vm.Destroy()
		return vm.RunBytecode(ctx, bytecode)
	}
	if !capture {
		return "", run()
	}
	// Moar caches stdout at instance create — redirect before Init.
	out, err := captureStdout(run)
	if err != nil {
		return out, err
	}
	return strings.TrimRight(out, "\r\n"), nil
}

// RunMoarEXE writes bytecode to a temp file and executes it with moar.exe.
func RunMoarEXE(bytecode []byte) (stdout string, err error) {
	moar := FindMoarEXE()
	if moar == "" {
		return "", fmt.Errorf("moarvm: neither moar.dll nor moar.exe found (place moar.dll next to the executable)")
	}
	tmp, err := os.CreateTemp("", "tcl_*.moarvm")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	if _, err := tmp.Write(bytecode); err != nil {
		tmp.Close()
		os.Remove(path)
		return "", err
	}
	tmp.Close()
	defer os.Remove(path)

	cmd := exec.Command(moar, path)
	cmd.Dir = filepath.Dir(moar)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("moar.exe: %w\n%s", err, out)
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}
