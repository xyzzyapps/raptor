package raptor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	moargo "moarvm-go/engine"
)

// Backend names: go (AST interpreter), moar (native CompUnit v7), wasm (TinyGo).
const (
	BackendGo   = "go"
	BackendMoar = "moar"
	BackendWASM = "wasm"
)

var defaultBackend = BackendGo

func SetDefaultBackend(name string) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case BackendMoar, "moarvm":
		defaultBackend = BackendMoar
	case BackendWASM, "tinygo":
		defaultBackend = BackendWASM
	default:
		defaultBackend = BackendGo
	}
}

func DefaultBackend() string { return defaultBackend }

// EvalOnBackend runs source with the selected backend.
func (in *Interp) EvalOnBackend(source, backend string) (*Value, error) {
	if backend == "" {
		backend = defaultBackend
	}
	switch backend {
	case BackendMoar:
		return in.evalMoar(source)
	case BackendWASM:
		in.GlobalEnv.Define("$*WASM", BoolValue(true))
		return in.Eval(source)
	default:
		return in.Eval(source)
	}
}

func (in *Interp) evalMoar(source string) (*Value, error) {
	c := NewCompiler()
	bc, err := c.CompileScript(source)
	if err != nil {
		// Full language still runs on the Go interpreter; Moar is used
		// whenever the subset compiler succeeds.
		return in.Eval(source)
	}
	out, err := moargo.RunNative(bc)
	if err != nil {
		return nil, fmt.Errorf("moar backend: %w", err)
	}
	if in.Stdout != nil && out != "" {
		fmt.Fprint(in.Stdout, out)
	}
	return StringValue(strings.TrimRight(out, "\r\n")), nil
}

// CompileToWASM invokes TinyGo (preferred) or go wasm to emit raptor.wasm.
func CompileToWASM(outPath string) error {
	if outPath == "" {
		outPath = filepath.Join("web", "raptor.wasm")
	}
	root := findModuleRoot()
	wasmMain := filepath.Join(root, "cmd", "wasm")
	if _, err := os.Stat(wasmMain); err != nil {
		return fmt.Errorf("cmd/wasm not found under %s", root)
	}
	if _, err := exec.LookPath("tinygo"); err == nil {
		cmd := exec.Command("tinygo", "build", "-target=wasm", "-no-debug", "-o", outPath, wasmMain)
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), "RAPTOR_WASM=1")
		return cmd.Run()
	}
	cmd := exec.Command("go", "build", "-o", outPath, wasmMain)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "RAPTOR_WASM=1")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go wasm build: %w\n%s", err, buf.String())
	}
	return nil
}

func findModuleRoot() string {
	wd, _ := os.Getwd()
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		p := filepath.Dir(dir)
		if p == dir {
			break
		}
		dir = p
	}
	return wd
}
