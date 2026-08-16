package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"moarvm-go/engine"
	"moarvm-go/tcl"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dllFlag := flag.String("dll", "", "Path to moar.dll")
	outFlag := flag.String("o", "", "Write compiled CompUnit v7 bytecode to this .moarvm file")
	emitOnly := flag.Bool("emit-only", false, "Parse and compile to bytecode, do not run")
	flag.Parse()

	args := flag.Args()
	dllPath := resolveDLL(*dllFlag)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if len(args) == 0 {
		runREPL(dllPath, logger)
		return
	}

	scriptPath := args[0]
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not read script %q: %v\n", scriptPath, err)
		os.Exit(1)
	}
	script := string(content)

	compiler := tcl.NewCompiler()
	bc, compileErr := compiler.CompileScript(script)

	if compileErr != nil {
		fmt.Fprintf(os.Stderr, "compile error: %v\n", compileErr)
		os.Exit(1)
	}

	if *outFlag != "" {
		if err := tcl.WriteCompUnit(*outFlag, bc); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", *outFlag, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", *outFlag, len(bc))
	}

	if *emitOnly {
		return
	}

	if dllPath != "" {
		os.Setenv("MOAR_DLL", dllPath)
	}
	if err := moargo.ExecNative(bc); err != nil {
		fmt.Fprintf(os.Stderr, "MoarVM: %v\n", err)
		os.Exit(1)
	}
}

func runREPL(dllPath string, logger *slog.Logger) {
	fmt.Println("Tcl on MoarVM (Interactive Shell)")
	fmt.Println("Type 'exit' or 'quit' to exit.")

	in := tcl.NewInterp()

	if dllPath != "" {
		vm, err := moargo.New(moargo.Config{
			DLLPath: dllPath,
			Logger:  logger,
		})
		if err == nil {
			ctx := context.Background()
			if err := vm.Init(ctx); err == nil {
				defer vm.Destroy()
				tcl.NewBridge(in, vm)
				fmt.Printf("Bound native MoarVM at %s\n", dllPath)
			}
		}
	}


	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("tcl> ")
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

		res, err := in.Eval(line)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else if res != "" {
			fmt.Println(res)
		}
	}
}

func resolveDLL(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if found := moargo.FindMoarDLL(); found != "" {
		return found
	}
	candidates := []string{
		"bin/moar.dll",
		"vendor/MoarVM/moar.dll",
		"build/moarvm/bin/moar.dll",
		"../build/moarvm/bin/moar.dll",
		"../../build/moarvm/bin/moar.dll",
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
