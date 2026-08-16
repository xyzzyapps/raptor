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

// WASM compiler names: "tinygo", "go", or "" (auto: TinyGo if on PATH).
var wasmCompiler = ""

func SetWASMCompiler(name string) {
	wasmCompiler = strings.ToLower(strings.TrimSpace(name))
}

func ResolveWASMCompiler() string {
	switch wasmCompiler {
	case "go", "gc":
		return "go"
	case "tinygo":
		return "tinygo"
	}
	if _, err := exec.LookPath("tinygo"); err == nil {
		return "tinygo"
	}
	return "go"
}

// CompileToWASM invokes TinyGo (preferred when available) or the Go
// wasm toolchain. On success it also writes raptor_bridge.js and
// wasm_exec.js next to the .wasm so a page can load the stubs without
// copying them by hand.
func CompileToWASM(outPath string) error {
	if outPath == "" {
		outPath = filepath.Join("web", "raptor.wasm")
	}
	root := findModuleRoot()
	wasmMain := filepath.Join(root, "cmd", "wasm")
	if _, err := os.Stat(wasmMain); err != nil {
		return fmt.Errorf("cmd/wasm not found under %s", root)
	}
	compiler := ResolveWASMCompiler()
	var err error
	if compiler == "tinygo" {
		err = runTinyGoWASM(root, outPath)
		if err != nil && wasmCompiler == "" {
			fmt.Fprintf(os.Stderr, "tinygo wasm failed (%v); falling back to go\n", err)
			err = runGoWASM(root, outPath)
			compiler = "go"
		}
	} else {
		err = runGoWASM(root, outPath)
	}
	if err != nil {
		return err
	}
	if werr := writeWASMStubs(root, filepath.Dir(outPath)); werr != nil {
		fmt.Fprintf(os.Stderr, "wasm stubs: %v\n", werr)
	}
	fmt.Fprintf(os.Stderr, "wasm compiler: %s -> %s\n", compiler, outPath)
	return nil
}

func runTinyGoWASM(root, outPath string) error {
	cmd := exec.Command("tinygo", "build", "-target=wasm", "-no-debug", "-o", outPath, filepath.Join(root, "cmd", "wasm"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "RAPTOR_WASM=1")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tinygo wasm: %w\n%s", err, buf.String())
	}
	return nil
}

func runGoWASM(root, outPath string) error {
	cmd := exec.Command("go", "build", "-o", outPath, filepath.Join(root, "cmd", "wasm"))
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

func writeWASMStubs(root, destDir string) error {
	if destDir == "" || destDir == "." {
		destDir = filepath.Join(root, "web")
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	bridgeSrc := filepath.Join(root, "runtime", "embedjs", "raptor_bridge.js")
	if b, err := os.ReadFile(bridgeSrc); err == nil {
		_ = os.WriteFile(filepath.Join(destDir, "raptor_bridge.js"), b, 0644)
	}
	// Prefer the project's wasm_exec.js; fall back to GOROOT.
	execSrc := filepath.Join(root, "web", "wasm_exec.js")
	if _, err := os.Stat(execSrc); err != nil {
		goroot := os.Getenv("GOROOT")
		if goroot == "" {
			if out, e := exec.Command("go", "env", "GOROOT").Output(); e == nil {
				goroot = strings.TrimSpace(string(out))
			}
		}
		for _, cand := range []string{
			filepath.Join(goroot, "lib", "wasm", "wasm_exec.js"),
			filepath.Join(goroot, "misc", "wasm", "wasm_exec.js"),
		} {
			if _, e := os.Stat(cand); e == nil {
				execSrc = cand
				break
			}
		}
	}
	if b, err := os.ReadFile(execSrc); err == nil {
		_ = os.WriteFile(filepath.Join(destDir, "wasm_exec.js"), b, 0644)
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
