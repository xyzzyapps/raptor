package main

import (
	"bufio"
	"flag"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"raptor/runtime"
	goruntime "runtime"
	"strings"
)

const version = "1.0.0 (Raptor Runtime - MoarVM JIT)"

func main() {
	dllFlag := flag.String("dll", "", "Path to moar.dll")
	outFlag := flag.String("o", "", "Output binary/bytecode file path")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging")
	evalFlag := flag.String("e", "", "Evaluate inline code string")
	goFlag := flag.Bool("go", false, "Use the Go interpreter backend")
	moarFlag := flag.Bool("moar", false, "Use the native MoarVM opcode backend")
	wasmFlag := flag.Bool("wasm", false, "Build/run via TinyGo WASM and enable WASM FFI helpers")
	backendFlag := flag.String("backend", "", "Execution backend: go, moar, or wasm")
	flag.Parse()

	backend := "go"
	if *backendFlag != "" {
		backend = strings.ToLower(*backendFlag)
	}
	if *wasmFlag {
		backend = "wasm"
	} else if *moarFlag {
		backend = "moar"
	} else if *goFlag {
		backend = "go"
	}
	raptor.SetDefaultBackend(backend)

	logLevel := slog.LevelWarn
	if *verboseFlag {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	if *evalFlag != "" {
		if backend == "wasm" {
			if err := raptor.CompileToWASM(*outFlag); err != nil {
				fmt.Fprintf(os.Stderr, "wasm build: %v\n", err)
				os.Exit(1)
			}
		}
		runInline(*evalFlag, *dllFlag, logger)
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		runREPL(*dllFlag, logger)
		return
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "run":
		if len(cmdArgs) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: raptor run <script.rp|script.raptor> [args...]\n")
			os.Exit(1)
		}
		runScript(cmdArgs[0], cmdArgs[1:], *dllFlag, logger)

	case "compile":
		if len(cmdArgs) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: raptor compile <source.rp> [-o output.moarvm]\n")
			os.Exit(1)
		}
		compileScript(cmdArgs[0], *outFlag)

	case "pack":
		if len(cmdArgs) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: raptor pack <script.rp|script.raptor> [-o output.exe]\n")
			os.Exit(1)
		}
		// Handle -o in cmdArgs if passed after pack
		targetScript := cmdArgs[0]
		targetOut := *outFlag
		for i := 0; i < len(cmdArgs); i++ {
			if (cmdArgs[i] == "-o" || cmdArgs[i] == "--output") && i+1 < len(cmdArgs) {
				targetOut = cmdArgs[i+1]
				break
			}
		}
		packScript(targetScript, targetOut)

	case "test":
		runTests(cmdArgs, logger)

	case "weave":
		runWeave(cmdArgs)

	case "tangle":
		runTangle(cmdArgs)

	case "stitch":
		runStitch(cmdArgs)

	case "init":
		name := ""
		if len(cmdArgs) > 0 {
			name = cmdArgs[0]
		}
		if err := raptor.InitPackage(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "get":
		if len(cmdArgs) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: raptor get <repo-url|user/repo>[@tag]\n")
			os.Exit(1)
		}
		if err := raptor.GetPackage(cmdArgs[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "install":
		if err := raptor.InstallPackages(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "serve":
		runServe(cmdArgs)

	case "doc", "perldoc", "help":
		topic := ""
		if len(cmdArgs) > 0 {
			topic = cmdArgs[0]
		}
		runDoc(topic)

	case "version", "-v", "--version":
		fmt.Printf("Raptor version %s\n", version)

	default:
		if strings.HasSuffix(command, ".rp") || strings.HasSuffix(command, ".raptor") || strings.HasSuffix(command, ".raku") || strings.HasSuffix(command, ".p6") || strings.HasSuffix(command, ".t") || strings.HasSuffix(command, ".pod") || strings.HasSuffix(command, ".lit.rp") {
			runScript(command, cmdArgs, *dllFlag, logger)
		} else {
			fmt.Fprintf(os.Stderr, "Unknown command or file: %s\n", command)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Printf("Raptor - High-Performance Language Runtime (Version %s)\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  raptor <script.rp|file.pod> [args...] Run script or literate POD")
	fmt.Println("  raptor run <script.rp|file.pod>       Run script or literate POD")
	fmt.Println("  raptor init [package_name]           Initialize new raptor.json package")
	fmt.Println("  raptor get <repo-url>[@tag]          Clone Git package into raptor_modules/")
	fmt.Println("  raptor install                       Install dependencies from raptor.json")
	fmt.Println("  raptor --go | --moar | --wasm        Select backend (Go interp, native MoarVM, TinyGo WASM)")
	fmt.Println("  raptor serve [--port 8080]           Launch WebAssembly In-Browser REPL & Playground")
	fmt.Println("  raptor weave <file.pod> [-o doc.md]   Weave literate POD to Markdown")
	fmt.Println("  raptor tangle <file.pod> [-o out_dir] Tangle literate POD to source code files")
	fmt.Println("  raptor stitch <file.pod> <source>     Stitch modified source back into literate POD")
	fmt.Println("  raptor test [t/*.t]                  Run TAP test harness (like prove)")
	fmt.Println("  raptor doc [topic]                   Read markdown manual documentation")
	fmt.Println("  raptor pack <script.rp> -o app.exe   Package script into standalone executable")
	fmt.Println("  raptor -e 'say 42;'                  Evaluate inline code")
	fmt.Println("  raptor compile <source> -o out       Compile script to MoarVM bytecode")
	fmt.Println("  raptor version                       Display version information")
	fmt.Println("  raptor                               Start interactive REPL")
}

func runInline(code, dllPath string, logger *slog.Logger) {
	in := raptor.NewInterp()
	val, err := in.EvalOnBackend(code, raptor.DefaultBackend())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}
	if val != nil && val.Type != raptor.ValNil {
		fmt.Println(val)
	}
}

func runScript(path string, scriptArgs []string, dllPath string, logger *slog.Logger) {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not read script %q: %v\n", path, err)
		os.Exit(1)
	}

	sourceCode := string(content)
	if strings.HasSuffix(path, ".pod") || strings.HasSuffix(path, ".lit.rp") || strings.HasSuffix(path, ".lit.raptor") {
		doc, err := raptor.ParsePodDoc(sourceCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Pod parse error: %v\n", err)
			os.Exit(1)
		}
		tangled, err := raptor.Tangle(doc, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Tangle error: %v\n", err)
			os.Exit(1)
		}
		if mainChunk, ok := doc.Chunks["main"]; ok {
			target := mainChunk.FileTarget
			if target == "" {
				target = "main.rp"
			}
			if code, ok := tangled[target]; ok {
				sourceCode = code
			} else if code, ok := tangled["main.rp"]; ok {
				sourceCode = code
			}
		} else if mainCode, ok := tangled["main.rp"]; ok {
			sourceCode = mainCode
		} else {
			for _, code := range tangled {
				sourceCode = code
				break
			}
		}
	}

	in := raptor.NewInterp()
	in.GlobalEnv.Define("$*PROGRAM", raptor.StringValue(path))
	in.GlobalEnv.Define("$*PROGRAM-NAME", raptor.StringValue(path))
	in.GlobalEnv.Define("$0", raptor.StringValue(path))
	var argsList []*raptor.Value
	for _, a := range scriptArgs {
		argsList = append(argsList, raptor.StringValue(a))
	}
	in.GlobalEnv.Define("@*ARGS", raptor.ArrayValue(argsList))
	in.GlobalEnv.Define("@ARGV", raptor.ArrayValue(argsList))

	if raptor.DefaultBackend() == raptor.BackendWASM {
		if err := raptor.CompileToWASM(""); err != nil {
			fmt.Fprintf(os.Stderr, "wasm build: %v\n", err)
			os.Exit(1)
		}
	}
	val, err := in.EvalOnBackend(sourceCode, raptor.DefaultBackend())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}
	if val != nil && val.Type != raptor.ValNil {
		fmt.Println(val)
	}
}

func runREPL(dllPath string, logger *slog.Logger) {
	fmt.Printf("Raptor %s - Interactive REPL (type 'exit' to quit)\n", version)
	in := raptor.NewInterp()
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("raptor> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "exit" || line == "quit" {
			break
		}
		if line == "" {
			continue
		}

		val, err := in.Eval(line)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else if val != nil && val.Type != raptor.ValNil {
			fmt.Println(val.String())
		}
	}
}

func compileScript(srcPath, outPath string) {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading source %q: %v\n", srcPath, err)
		os.Exit(1)
	}

	if outPath == "" {
		ext := filepath.Ext(srcPath)
		outPath = strings.TrimSuffix(srcPath, ext) + ".moarvm"
	}

	compiler := raptor.NewCompiler()
	prog, err := raptor.ParseProgram(string(content))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	bc, err := compiler.CompileAST(prog.Stmts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, bc, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing bytecode to %q: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("Compiled %s -> %s (%d bytes)\n", srcPath, outPath, len(bc))
}

func packScript(srcPath, outPath string) {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading source %q: %v\n", srcPath, err)
		os.Exit(1)
	}

	if outPath == "" {
		ext := filepath.Ext(srcPath)
		outPath = strings.TrimSuffix(srcPath, ext)
		if goruntime.GOOS == "windows" {
			outPath += ".exe"
		}
	}

	absOut, err := filepath.Abs(outPath)
	if err != nil {
		absOut = outPath
	}

	outDir := filepath.Dir(absOut)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory %q: %v\n", outDir, err)
		os.Exit(1)
	}

	tempDir, err := os.MkdirTemp("", "raptor-pack-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temporary build dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	raptorRoot := findRaptorModuleRoot()
	parent := filepath.Dir(raptorRoot)
	moarvmRoot := filepath.Join(parent, "moarvm-go")
	gcreRoot := filepath.Join(parent, "gcre")

	goModContent := fmt.Sprintf(`module raptorapp

go 1.22

require (
	raptor v0.0.0
	moarvm-go v0.0.0
	gcre v0.0.0
)

replace raptor => %s
replace moarvm-go => %s
replace gcre => %s
`, strings.ReplaceAll(raptorRoot, "\\", "/"), strings.ReplaceAll(moarvmRoot, "\\", "/"), strings.ReplaceAll(gcreRoot, "\\", "/"))


	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing go.mod: %v\n", err)
		os.Exit(1)
	}

	mainGoContent := fmt.Sprintf(`package main

import (
	"fmt"
	"os"
	"raptor/runtime"
)

const embeddedScript = %q

func main() {
	in := raptor.NewInterp()
	val, err := in.Eval(embeddedScript)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %%v\n", err)
		os.Exit(1)
	}
	if val != nil && val.Type != raptor.ValNil {
		fmt.Println(val)
	}
}
`, string(content))

	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(mainGoContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing main.go: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "build", "-mod=mod", "-o", absOut, ".")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error building standalone binary: %v\n", err)
		os.Exit(1)
	}

	fi, err := os.Stat(absOut)
	sizeStr := ""
	if err == nil {
		sizeStr = fmt.Sprintf(" (%d bytes)", fi.Size())
	}
	fmt.Printf("Successfully packaged %s -> %s%s\n", srcPath, absOut, sizeStr)
}

func findRaptorModuleRoot() string {
	wd, err := os.Getwd()
	if err == nil {
		dir := wd
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				abs, _ := filepath.Abs(dir)
				return abs
			}
			if _, err := os.Stat(filepath.Join(dir, "raptor", "go.mod")); err == nil {
				abs, _ := filepath.Abs(filepath.Join(dir, "raptor"))
				return abs
			}
			parent := filepath.Dir(dir)
			if parent == dir || parent == "" {
				break
			}
			dir = parent
		}
	}

	exePath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exePath)
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				abs, _ := filepath.Abs(dir)
				return abs
			}
			if _, err := os.Stat(filepath.Join(dir, "raptor", "go.mod")); err == nil {
				abs, _ := filepath.Abs(filepath.Join(dir, "raptor"))
				return abs
			}
			parent := filepath.Dir(dir)
			if parent == dir || parent == "" {
				break
			}
			dir = parent
		}
	}

	if wd != "" {
		abs, _ := filepath.Abs(wd)
		return abs
	}
	return "."
}

func runTests(paths []string, logger *slog.Logger) {
	var testFiles []string
	if len(paths) == 0 {
		paths = []string{"t"}
	}
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			matches, _ := filepath.Glob(p)
			testFiles = append(testFiles, matches...)
			continue
		}
		if fi.IsDir() {
			entries, _ := os.ReadDir(p)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".t") || strings.HasSuffix(e.Name(), ".raku") {
					testFiles = append(testFiles, filepath.Join(p, e.Name()))
				}
			}
		} else {
			testFiles = append(testFiles, p)
		}
	}

	if len(testFiles) == 0 {
		fmt.Println("No test files found in specified paths.")
		return
	}

	os.Setenv("RAPTOR_TEST_MODE", "1")
	fmt.Printf("=== Running Raptor TAP Test Harness (%d files) ===\n\n", len(testFiles))

	totalPassedFiles := 0
	totalFailedFiles := 0
	totalTestsRun := 0
	totalTestsFailed := 0

	for _, tf := range testFiles {
		fmt.Printf("==> %s\n", tf)
		content, err := os.ReadFile(tf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed reading %s: %v\n", tf, err)
			totalFailedFiles++
			continue
		}

		in := raptor.NewInterp()
		_, err = in.EvalOnBackend(string(content), raptor.DefaultBackend())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Script error in %s: %v\n", tf, err)
			totalFailedFiles++
			continue
		}

		if summaryFn, ok := in.Builtins["tap_summary"]; ok {
			sVal, _ := summaryFn(in, nil)
			if sVal != nil && sVal.Type == raptor.ValHash {
				t := int(sVal.HashVal["total"].IntVal)
				f := int(sVal.HashVal["failed"].IntVal)
				totalTestsRun += t
				totalTestsFailed += f
				if f == 0 {
					totalPassedFiles++
				} else {
					totalFailedFiles++
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("--------------------------------------------------")
	if totalFailedFiles == 0 {
		fmt.Printf("Result: PASS (%d test files, %d total assertions passed)\n", len(testFiles), totalTestsRun)
	} else {
		fmt.Printf("Result: FAIL (%d/%d test files failed, %d total failed assertions)\n", totalFailedFiles, len(testFiles), totalTestsFailed)
		os.Exit(1)
	}
}

func runDoc(topic string) {
	if topic == "" {
		topic = "01_introduction"
	}
	docsDir := filepath.Join(findRaptorModuleRoot(), "docs")
	candidates := []string{
		filepath.Join(docsDir, topic+".pod"),
		filepath.Join(docsDir, topic+".md"),
		filepath.Join(docsDir, topic),
		filepath.Join(docsDir, "perlraptor.pod"),
		filepath.Join(docsDir, "01_introduction.md"),
	}

	entries, _ := os.ReadDir(docsDir)
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name()), strings.ToLower(topic)) {
			candidates = append([]string{filepath.Join(docsDir, e.Name())}, candidates...)
		}
	}

	for _, cand := range candidates {
		content, err := os.ReadFile(cand)
		if err == nil {
			in := raptor.NewInterp()
			if mdFn, ok := in.Builtins["tui_markdown"]; ok {
				out, _ := mdFn(in, []*raptor.Value{raptor.StringValue(string(content))})
				if out != nil {
					fmt.Println(out.String())
					return
				}
			}
			fmt.Println(string(content))
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Documentation topic %q not found in %s\nAvailable topics:\n", topic, docsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			fmt.Printf("  - %s\n", strings.TrimSuffix(e.Name(), ".md"))
		}
	}
}

func runWeave(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: raptor weave <file.pod> [-o output.md]\n")
		os.Exit(1)
	}
	srcPath := args[0]
	outPath := ""
	for i := 1; i < len(args); i++ {
		if (args[i] == "-o" || args[i] == "--output") && i+1 < len(args) {
			outPath = args[i+1]
			break
		}
	}
	content, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %q: %v\n", srcPath, err)
		os.Exit(1)
	}
	doc, err := raptor.ParsePodDoc(string(content))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Pod parse error: %v\n", err)
		os.Exit(1)
	}
	md := raptor.WeaveMarkdown(doc)
	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(md), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing markdown to %q: %v\n", outPath, err)
			os.Exit(1)
		}
		fmt.Printf("Wove %s -> %s (%d bytes)\n", srcPath, outPath, len(md))
	} else {
		fmt.Println(md)
	}
}

func runTangle(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: raptor tangle <file.pod> [-o out_dir | --target chunk_name]\n")
		os.Exit(1)
	}
	srcPath := args[0]
	outDir := "."
	targetFilter := ""
	for i := 1; i < len(args); i++ {
		if (args[i] == "-o" || args[i] == "--output" || args[i] == "--dir") && i+1 < len(args) {
			outDir = args[i+1]
		}
		if (args[i] == "-t" || args[i] == "--target") && i+1 < len(args) {
			targetFilter = args[i+1]
		}
	}
	content, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %q: %v\n", srcPath, err)
		os.Exit(1)
	}
	doc, err := raptor.ParsePodDoc(string(content))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Pod parse error: %v\n", err)
		os.Exit(1)
	}
	files, err := raptor.Tangle(doc, targetFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Tangle error: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No tangled target files found in %s\n", srcPath)
		return
	}
	for targetPath, code := range files {
		dest := filepath.Join(outDir, targetPath)
		_ = os.MkdirAll(filepath.Dir(dest), 0755)
		if err := os.WriteFile(dest, []byte(code), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", dest, err)
			continue
		}
		fmt.Printf("Tangled -> %s (%d bytes)\n", dest, len(code))
	}
}

func runStitch(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: raptor stitch <file.pod> <modified_source_file|dir> [-o output.pod]\n")
		os.Exit(1)
	}
	podPath := args[0]
	sourceInput := args[1]
	outPath := podPath // default in-place

	for i := 2; i < len(args); i++ {
		if (args[i] == "-o" || args[i] == "--output") && i+1 < len(args) {
			outPath = args[i+1]
			break
		}
	}

	podContent, err := os.ReadFile(podPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading POD file %q: %v\n", podPath, err)
		os.Exit(1)
	}

	filesMap := make(map[string]string)
	fi, err := os.Stat(sourceInput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading source input %q: %v\n", sourceInput, err)
		os.Exit(1)
	}

	if fi.IsDir() {
		_ = filepath.Walk(sourceInput, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".rp") || strings.HasSuffix(path, ".raptor") || strings.HasSuffix(path, ".c") || strings.HasSuffix(path, ".h")) {
				rel, _ := filepath.Rel(sourceInput, path)
				rel = filepath.ToSlash(rel)
				b, _ := os.ReadFile(path)
				filesMap[rel] = string(b)
				filesMap[filepath.Base(path)] = string(b)
			}
			return nil
		})
	} else {
		b, err := os.ReadFile(sourceInput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading source file %q: %v\n", sourceInput, err)
			os.Exit(1)
		}
		filesMap[filepath.Base(sourceInput)] = string(b)
		filesMap[filepath.ToSlash(sourceInput)] = string(b)
	}

	updatedPod, err := raptor.Stitch(string(podContent), filesMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Stitch error: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, []byte(updatedPod), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing updated POD to %q: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("Stitched %s <- %s -> %s (%d bytes)\n", podPath, sourceInput, outPath, len(updatedPod))
}

func runServe(cmdArgs []string) {
	port := "8080"
	for i := 0; i < len(cmdArgs); i++ {
		if (cmdArgs[i] == "-p" || cmdArgs[i] == "--port") && i+1 < len(cmdArgs) {
			port = cmdArgs[i+1]
			break
		}
	}

	webDir := "web"
	if root := findRaptorModuleRoot(); root != "" {
		cand := filepath.Join(root, "web")
		if info, err := os.Stat(cand); err == nil && info.IsDir() {
			webDir = cand
		}
	}

	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
	_ = mime.AddExtensionType(".html", "text/html")

	fileServer := http.FileServer(http.Dir(webDir))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fileServer.ServeHTTP(w, r)
	})

	addr := ":" + port
	fmt.Printf("=== Raptor WebAssembly REPL & IDE Server ===\n")
	fmt.Printf("Serving from: %s\n", webDir)
	fmt.Printf("Web Playground URL: http://localhost:%s/\n", port)
	fmt.Printf("Press Ctrl+C to terminate server.\n")

	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		os.Exit(1)
	}
}
