package main

import (
	"flag"
	"fmt"
	"os"
	"raptor/runtime"
)

const version = "1.0.0 (RaptorHP PHP-Style Template Engine)"

func main() {
	evalFlag := flag.String("r", "", "Run PHP-style template/code inline without tags")
	serverFlag := flag.String("S", "", "Run built-in development web server (e.g. localhost:8000)")
	docRootFlag := flag.String("t", ".", "Document root for -S (like php -S -t public)")
	versionFlag := flag.Bool("v", false, "Display version")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("RaptorHP Version %s\n", version)
		return
	}

	if *evalFlag != "" {
		runInlineCode(*evalFlag)
		return
	}

	if *serverFlag != "" {
		router := ""
		if extra := flag.Args(); len(extra) > 0 {
			router = extra[0]
		}
		if err := raptor.ServeRaptorHP(raptor.HPServerOptions{
			Addr:    *serverFlag,
			DocRoot: *docRootFlag,
			Router:  router,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		return
	}

	runTemplateFile(args[0])
}

func printUsage() {
	fmt.Printf("RaptorHP Template Engine & Server (Version %s)\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  raptorhp <file.phtml|file.html|file.rp>  Execute template file and render HTML to stdout")
	fmt.Println("  raptorhp -r '<h1><?= \"Hello\" ?></h1>'  Render inline template string")
	fmt.Println("  raptorhp -S localhost:8000               Start PHP-style HTTP server (cwd)")
	fmt.Println("  raptorhp -S localhost:8000 -t public     Serve from document root")
	fmt.Println("  raptorhp -S localhost:8000 router.phtml  Front-controller router")
	fmt.Println("  raptorhp -v                              Show version information")
	fmt.Println("  (same as: raptor -S localhost:8000 [-t dir] [router])")
}

func runInlineCode(templateSource string) {
	in := raptor.NewInterp()
	rendered, err := raptor.RenderTemplate(templateSource, in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Template Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(rendered)
}

func runTemplateFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %q: %v\n", path, err)
		os.Exit(1)
	}

	in := raptor.NewInterp()
	rendered, err := raptor.RenderTemplate(string(content), in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Template Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(rendered)
}
