//go:build !js || !wasm

package raptor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type httpServerInstance struct {
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
	closed   bool
}

var (
	httpServersMu sync.Mutex
	httpServers   = make(map[uintptr]*httpServerInstance)
	serverIDGen   uintptr
)

func (in *Interp) registerHTTPBuiltins() {
	// http_get(url, [headers])
	in.Builtins["http_get"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("http_get requires at least a URL argument")
		}
		url := args[0].String()
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid http request: %w", err)
		}

		if len(args) > 1 && args[1].Type == ValHash && args[1].HashVal != nil {
			for k, v := range args[1].HashVal {
				req.Header.Set(k, v.String())
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http_get request failed: %w", err)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed reading response body: %w", err)
		}

		headersHash := make(map[string]*Value)
		for k, v := range resp.Header {
			headersHash[k] = StringValue(strings.Join(v, ", "))
		}

		res := make(map[string]*Value)
		res["status"] = IntValue(int64(resp.StatusCode))
		res["body"] = StringValue(string(bodyBytes))
		res["headers"] = HashValue(headersHash)
		return HashValue(res), nil
	}

	// http_post(url, body, [headers])
	in.Builtins["http_post"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("http_post requires URL and body arguments")
		}
		url := args[0].String()
		bodyStr := args[1].String()

		req, err := http.NewRequest("POST", url, strings.NewReader(bodyStr))
		if err != nil {
			return nil, fmt.Errorf("invalid http request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		if len(args) > 2 && args[2].Type == ValHash && args[2].HashVal != nil {
			for k, v := range args[2].HashVal {
				req.Header.Set(k, v.String())
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http_post request failed: %w", err)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed reading response body: %w", err)
		}

		headersHash := make(map[string]*Value)
		for k, v := range resp.Header {
			headersHash[k] = StringValue(strings.Join(v, ", "))
		}

		res := make(map[string]*Value)
		res["status"] = IntValue(int64(resp.StatusCode))
		res["body"] = StringValue(string(bodyBytes))
		res["headers"] = HashValue(headersHash)
		return HashValue(res), nil
	}

	// http_request(method, url, [body], [headers])
	in.Builtins["http_request"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("http_request requires method and URL arguments")
		}
		method := strings.ToUpper(args[0].String())
		url := args[1].String()

		var bodyReader io.Reader
		if len(args) > 2 && args[2].Type != ValNil {
			bodyReader = strings.NewReader(args[2].String())
		}

		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("invalid http request: %w", err)
		}

		if len(args) > 3 && args[3].Type == ValHash && args[3].HashVal != nil {
			for k, v := range args[3].HashVal {
				req.Header.Set(k, v.String())
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed reading response body: %w", err)
		}

		headersHash := make(map[string]*Value)
		for k, v := range resp.Header {
			headersHash[k] = StringValue(strings.Join(v, ", "))
		}

		res := make(map[string]*Value)
		res["status"] = IntValue(int64(resp.StatusCode))
		res["body"] = StringValue(string(bodyBytes))
		res["headers"] = HashValue(headersHash)
		return HashValue(res), nil
	}

	// http_server_listen(port, handler_callable)
	in.Builtins["http_server_listen"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("http_server_listen requires port and handler callable arguments")
		}
		port := args[0].String()
		if !strings.Contains(port, ":") {
			port = ":" + port
		}
		handlerVal := args[1]
		if handlerVal.Type != ValClosure && handlerVal.Type != ValMultiSub {
			return nil, fmt.Errorf("http_server_listen: expected callable handler, got %s", handlerVal.TypeName())
		}

		listener, err := net.Listen("tcp", port)
		if err != nil {
			return nil, fmt.Errorf("failed to bind HTTP server to %s: %w", port, err)
		}

		server := &http.Server{}
		inst := &httpServerInstance{
			server:   server,
			listener: listener,
		}

		httpServersMu.Lock()
		serverIDGen++
		handleID := serverIDGen
		httpServers[handleID] = inst
		httpServersMu.Unlock()

		server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqBody, _ := io.ReadAll(r.Body)

			reqHeaders := make(map[string]*Value)
			for k, v := range r.Header {
				reqHeaders[k] = StringValue(strings.Join(v, ", "))
			}

			reqQuery := make(map[string]*Value)
			for k, v := range r.URL.Query() {
				reqQuery[k] = StringValue(strings.Join(v, ", "))
			}

			reqHash := make(map[string]*Value)
			reqHash["method"] = StringValue(r.Method)
			reqHash["path"] = StringValue(r.URL.Path)
			reqHash["body"] = StringValue(string(reqBody))
			reqHash["headers"] = HashValue(reqHeaders)
			reqHash["query"] = HashValue(reqQuery)

			resVal, err := in.InvokeCallable(handlerVal, []*Value{HashValue(reqHash)})
			if err != nil {
				http.Error(w, fmt.Sprintf("Internal Server Error: %v", err), http.StatusInternalServerError)
				return
			}

			statusCode := http.StatusOK
			var resBody []byte

			if resVal.Type == ValHash && resVal.HashVal != nil {
				if s, ok := resVal.HashVal["status"]; ok {
					statusCode = int(in.toInt(s))
				}
				if h, ok := resVal.HashVal["headers"]; ok && h.Type == ValHash && h.HashVal != nil {
					for hk, hv := range h.HashVal {
						w.Header().Set(hk, hv.String())
					}
				}
				if b, ok := resVal.HashVal["body"]; ok {
					resBody = []byte(b.String())
				}
			} else {
				resBody = []byte(resVal.String())
			}

			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			}

			w.WriteHeader(statusCode)
			w.Write(resBody)
		})

		go func() {
			_ = server.Serve(listener)
		}()

		resObj := make(map[string]*Value)
		resObj["handle"] = NativePtrValue(handleID)
		resObj["port"] = StringValue(port)
		resObj["close"] = ClosureValue(&Closure{
			Name: "http_server_close",
			Body: &BlockStmt{},
			Env:  in.GlobalEnv,
		})

		return HashValue(resObj), nil
	}

	// http_server_close(server_handle)
	in.Builtins["http_server_close"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		var handleID uintptr
		if args[0].Type == ValNativePtr {
			handleID = args[0].PtrVal
		} else if args[0].Type == ValHash && args[0].HashVal != nil {
			if h, ok := args[0].HashVal["handle"]; ok && h.Type == ValNativePtr {
				handleID = h.PtrVal
			}
		}

		httpServersMu.Lock()
		inst, ok := httpServers[handleID]
		if ok {
			delete(httpServers, handleID)
		}
		httpServersMu.Unlock()

		if !ok || inst == nil {
			return BoolValue(false), nil
		}

		inst.mu.Lock()
		defer inst.mu.Unlock()
		if !inst.closed {
			inst.closed = true
			_ = inst.server.Shutdown(context.Background())
			_ = inst.listener.Close()
		}
		return BoolValue(true), nil
	}

	// http_parse_request(raw_text) -> { method, path, headers, body, query }
	in.Builtins["http_parse_request"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("http_parse_request requires raw HTTP text")
		}
		raw := args[0].String()
		lines := strings.Split(raw, "\r\n")
		if len(lines) == 1 {
			lines = strings.Split(raw, "\n")
		}
		if len(lines) == 0 || lines[0] == "" {
			return nil, fmt.Errorf("empty HTTP request")
		}

		reqLine := strings.Fields(lines[0])
		method := "GET"
		path := "/"
		if len(reqLine) >= 1 {
			method = reqLine[0]
		}
		if len(reqLine) >= 2 {
			path = reqLine[1]
		}

		headers := make(map[string]*Value)
		bodyStart := len(lines)
		for i := 1; i < len(lines); i++ {
			line := lines[i]
			if line == "" {
				bodyStart = i + 1
				break
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				headers[strings.TrimSpace(parts[0])] = StringValue(strings.TrimSpace(parts[1]))
			}
		}

		body := ""
		if bodyStart < len(lines) {
			body = strings.Join(lines[bodyStart:], "\n")
		}

		res := make(map[string]*Value)
		res["method"] = StringValue(method)
		res["path"] = StringValue(path)
		res["headers"] = HashValue(headers)
		res["body"] = StringValue(body)
		return HashValue(res), nil
	}

	// http_format_response(status_code, headers_hash, body_str) -> formatted HTTP response string
	in.Builtins["http_format_response"] = func(in *Interp, args []*Value) (*Value, error) {
		statusCode := int64(200)
		if len(args) >= 1 {
			statusCode = in.toInt(args[0])
		}
		statusText := http.StatusText(int(statusCode))
		if statusText == "" {
			statusText = "OK"
		}

		body := ""
		if len(args) >= 3 {
			body = args[2].String()
		} else if len(args) == 2 && args[1].Type == ValString {
			body = args[1].String()
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, statusText))
		hasLen := false
		hasType := false

		if len(args) >= 2 && args[1].Type == ValHash && args[1].HashVal != nil {
			for k, v := range args[1].HashVal {
				if strings.EqualFold(k, "Content-Length") {
					hasLen = true
				}
				if strings.EqualFold(k, "Content-Type") {
					hasType = true
				}
				sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v.String()))
			}
		}

		if !hasType {
			sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		}
		if !hasLen {
			sb.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(body)))
		}
		sb.WriteString("\r\n")
		sb.WriteString(body)

		return StringValue(sb.String()), nil
	}
}
