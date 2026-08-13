package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"raptor/runtime"
	"strings"
)

const version = "1.0.0 (RaptorHP PHP-Style Template Engine)"

func main() {
	evalFlag := flag.String("r", "", "Run PHP-style template/code inline without tags")
	serverFlag := flag.String("S", "", "Run built-in development web server (e.g. localhost:8000)")
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
		runDevServer(*serverFlag)
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
	fmt.Println("  raptorhp -S localhost:8000               Start built-in development HTTP template server")
	fmt.Println("  raptorhp -v                              Show version information")
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

func runDevServer(addr string) {
	fmt.Printf("=== RaptorHP Development Web Server ===\n")
	fmt.Printf("Listening on http://%s/\n", addr)
	fmt.Printf("Document Root is: %s\n", getCurrentDir())
	fmt.Printf("Press Ctrl+C to stop server.\n\n")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path
		if reqPath == "/" {
			// Look for index.phtml, index.rhtml, index.html, index.php
			candidates := []string{"index.phtml", "index.rhtml", "index.rp", "index.php", "index.html"}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					reqPath = "/" + c
					break
				}
			}
		}

		filePath := filepath.Join(".", filepath.Clean(reqPath))
		info, err := os.Stat(filePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if info.IsDir() {
			http.NotFound(w, r)
			return
		}

		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".phtml" || ext == ".rhtml" || ext == ".rp" || ext == ".raptor" || ext == ".php" || ext == ".html" {
			content, err := os.ReadFile(filePath)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error reading file: %v", err), http.StatusInternalServerError)
				return
			}

			in := raptor.NewInterp()

			// Populate superglobals in interpreter
			getMap := make(map[string]*raptor.Value)
			for k, v := range r.URL.Query() {
				if len(v) > 0 {
					getMap[k] = raptor.StringValue(v[0])
				}
			}
			in.GlobalEnv.Define("%_GET", raptor.HashValue(getMap))
			in.GlobalEnv.Define("%*GET", raptor.HashValue(getMap))

			serverMap := make(map[string]*raptor.Value)
			serverMap["REQUEST_METHOD"] = raptor.StringValue(r.Method)
			serverMap["REQUEST_URI"] = raptor.StringValue(r.RequestURI)
			serverMap["REMOTE_ADDR"] = raptor.StringValue(r.RemoteAddr)
			in.GlobalEnv.Define("%_SERVER", raptor.HashValue(serverMap))
			in.GlobalEnv.Define("%*SERVER", raptor.HashValue(serverMap))

			rendered, err := raptor.RenderTemplate(string(content), in)
			if err != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "<h3>RaptorHP Template Error</h3><pre>%v</pre>", err)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("X-Powered-By", "RaptorHP 1.0")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, rendered)
		} else {
			// Serve static asset
			http.ServeFile(w, r, filePath)
		}
	})

	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed to start: %v\n", err)
		os.Exit(1)
	}
}

func getCurrentDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
