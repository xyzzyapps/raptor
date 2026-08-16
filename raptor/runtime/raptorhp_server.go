package raptor

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HPServerOptions is the PHP-style built-in server (`raptor -S`, `raptorhp -S`).
type HPServerOptions struct {
	Addr    string // host:port, e.g. "localhost:8000"
	DocRoot string // -t document root (default ".")
	Router  string // optional router script, like php -S ... router.php
}

func isHPTemplate(ext string) bool {
	switch strings.ToLower(ext) {
	case ".phtml", ".rhtml", ".rp", ".raptor", ".php", ".html", ".rphp":
		return true
	default:
		return false
	}
}

func parsePOST(r *http.Request) map[string]*Value {
	out := make(map[string]*Value)
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/x-www-form-urlencoded") || strings.HasPrefix(ct, "multipart/form-data") {
		_ = r.ParseMultipartForm(8 << 20)
		_ = r.ParseForm()
		for k, v := range r.PostForm {
			if len(v) > 0 {
				out[k] = StringValue(v[0])
			}
		}
	}
	return out
}

func bindHPSuperglobals(in *Interp, r *http.Request) {
	getMap := make(map[string]*Value)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			getMap[k] = StringValue(v[0])
		}
	}
	in.GlobalEnv.Define("%_GET", HashValue(getMap))
	in.GlobalEnv.Define("%*GET", HashValue(getMap))

	postMap := parsePOST(r)
	in.GlobalEnv.Define("%_POST", HashValue(postMap))
	in.GlobalEnv.Define("%*POST", HashValue(postMap))

	serverMap := map[string]*Value{
		"REQUEST_METHOD":  StringValue(r.Method),
		"REQUEST_URI":     StringValue(r.RequestURI),
		"REMOTE_ADDR":     StringValue(r.RemoteAddr),
		"SERVER_SOFTWARE": StringValue("RaptorHP/1.0"),
		"DOCUMENT_ROOT":   StringValue(mustAbs(".")),
		"SCRIPT_NAME":     StringValue(r.URL.Path),
		"QUERY_STRING":    StringValue(r.URL.RawQuery),
		"HTTP_HOST":       StringValue(r.Host),
	}
	in.GlobalEnv.Define("%_SERVER", HashValue(serverMap))
	in.GlobalEnv.Define("%*SERVER", HashValue(serverMap))
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

func renderHPFile(w http.ResponseWriter, r *http.Request, filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading file: %v", err), http.StatusInternalServerError)
		return
	}
	in := NewInterp()
	bindHPSuperglobals(in, r)
	rendered, err := RenderTemplate(string(content), in)
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
}

func listDirectory(w http.ResponseWriter, dir, reqPath string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<h1>Index of %s</h1><ul>", reqPath)
	if reqPath != "/" {
		fmt.Fprintf(w, `<li><a href="..">..</a></li>`)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		fmt.Fprintf(w, `<li><a href="%s">%s</a></li>`, name, name)
	}
	fmt.Fprintf(w, "</ul>")
}

// ServeRaptorHP starts a PHP-like development server.
// raptor -S localhost:8000   /  raptorhp -S localhost:8000
func ServeRaptorHP(opts HPServerOptions) error {
	if opts.Addr == "" {
		opts.Addr = "localhost:8000"
	}
	if opts.DocRoot == "" {
		opts.DocRoot = "."
	}
	root, err := filepath.Abs(opts.DocRoot)
	if err != nil {
		return err
	}
	fmt.Printf("=== RaptorHP Development Web Server (PHP -S compatible) ===\n")
	fmt.Printf("Listening on http://%s/\n", opts.Addr)
	fmt.Printf("Document Root is: %s\n", root)
	if opts.Router != "" {
		fmt.Printf("Router: %s\n", opts.Router)
	}
	fmt.Printf("Press Ctrl+C to stop server.\n\n")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if opts.Router != "" {
			router := opts.Router
			if !filepath.IsAbs(router) {
				router = filepath.Join(root, router)
			}
			if _, err := os.Stat(router); err == nil {
				renderHPFile(w, r, router)
				return
			}
		}

		reqPath := r.URL.Path
		if reqPath == "/" {
			for _, c := range []string{"index.phtml", "index.rhtml", "index.rphp", "index.rp", "index.php", "index.html"} {
				if _, err := os.Stat(filepath.Join(root, c)); err == nil {
					reqPath = "/" + c
					break
				}
			}
		}

		filePath := filepath.Join(root, filepath.Clean("/"+reqPath))
		info, err := os.Stat(filePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			for _, c := range []string{"index.phtml", "index.rhtml", "index.html"} {
				idx := filepath.Join(filePath, c)
				if _, err := os.Stat(idx); err == nil {
					renderHPFile(w, r, idx)
					return
				}
			}
			listDirectory(w, filePath, r.URL.Path)
			return
		}
		if isHPTemplate(filepath.Ext(filePath)) {
			renderHPFile(w, r, filePath)
			return
		}
		http.ServeFile(w, r, filePath)
	})
	return http.ListenAndServe(opts.Addr, mux)
}
