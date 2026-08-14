//go:build !js || !wasm

package raptor

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type wsConnInstance struct {
	conn       net.Conn
	reader     *bufio.Reader
	mu         sync.Mutex
	closed     bool
	isClient   bool
	onMsg      *Value
	in         *Interp
	recvChan   chan *Value
}

var (
	wsConnsMu sync.Mutex
	wsConns   = make(map[uintptr]*wsConnInstance)
	wsIDGen   uintptr
)

func (in *Interp) registerWebSocketBuiltins() {
	// ws_listen(port, path, on_connect_callable)
	in.Builtins["ws_listen"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("ws_listen requires port and on_connect callable arguments")
		}
		port := args[0].String()
		if !strings.Contains(port, ":") {
			port = ":" + port
		}

		var path string = "/"
		var onConnectVal *Value
		if len(args) == 2 {
			onConnectVal = args[1]
		} else {
			path = args[1].String()
			onConnectVal = args[2]
		}

		if onConnectVal.Type != ValClosure && onConnectVal.Type != ValMultiSub {
			return nil, fmt.Errorf("ws_listen: expected callable on_connect handler, got %s", onConnectVal.TypeName())
		}

		listener, err := net.Listen("tcp", port)
		if err != nil {
			return nil, fmt.Errorf("failed to bind WebSocket listener to %s: %w", port, err)
		}

		mux := http.NewServeMux()
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				http.Error(w, "Expected WebSocket Upgrade", http.StatusBadRequest)
				return
			}

			key := r.Header.Get("Sec-WebSocket-Key")
			if key == "" {
				http.Error(w, "Missing Sec-WebSocket-Key", http.StatusBadRequest)
				return
			}

			// RFC 6455 Handshake
			acceptKey := computeWSAcceptKey(key)
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
				return
			}

			conn, bufrw, err := hj.Hijack()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			response := "HTTP/1.1 101 Switching Protocols\r\n" +
				"Upgrade: websocket\r\n" +
				"Connection: Upgrade\r\n" +
				"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
			conn.Write([]byte(response))

			wsInst := &wsConnInstance{
				conn:     conn,
				reader:   bufrw.Reader,
				isClient: false,
				in:       in,
				recvChan: make(chan *Value, 64),
			}

			wsConnsMu.Lock()
			wsIDGen++
			cID := wsIDGen
			wsConns[cID] = wsInst
			wsConnsMu.Unlock()

			wsVal := in.createWSValue(cID, wsInst)

			// Spawn read loop
			go wsInst.readLoop()

			// Call on_connect
			_, _ = in.InvokeCallable(onConnectVal, []*Value{wsVal})
		})

		server := &http.Server{Handler: mux}
		go func() {
			_ = server.Serve(listener)
		}()

		resObj := make(map[string]*Value)
		resObj["port"] = StringValue(port)
		resObj["path"] = StringValue(path)
		return HashValue(resObj), nil
	}

	// ws_connect(url)
	in.Builtins["ws_connect"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("ws_connect requires a target URL")
		}
		rawURL := args[0].String()
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid websocket url: %w", err)
		}

		host := u.Host
		if !strings.Contains(host, ":") {
			if u.Scheme == "wss" {
				host += ":443"
			} else {
				host += ":80"
			}
		}

		conn, err := net.Dial("tcp", host)
		if err != nil {
			return nil, fmt.Errorf("failed connecting to %s: %w", host, err)
		}

		// Generate random 16-byte key
		keyBytes := make([]byte, 16)
		_, _ = rand.Read(keyBytes)
		secKey := base64.StdEncoding.EncodeToString(keyBytes)

		reqPath := u.Path
		if reqPath == "" {
			reqPath = "/"
		}
		if u.RawQuery != "" {
			reqPath += "?" + u.RawQuery
		}

		req := fmt.Sprintf("GET %s HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n\r\n", reqPath, u.Host, secKey)

		if _, err := conn.Write([]byte(req)); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("handshake request failed: %w", err)
		}

		reader := bufio.NewReader(conn)
		resp, err := http.ReadResponse(reader, &http.Request{Method: "GET"})
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("handshake response failed: %w", err)
		}
		if resp.StatusCode != 101 {
			_ = conn.Close()
			return nil, fmt.Errorf("websocket handshake rejected with status: %d", resp.StatusCode)
		}

		wsInst := &wsConnInstance{
			conn:     conn,
			reader:   reader,
			isClient: true,
			in:       in,
			recvChan: make(chan *Value, 64),
		}

		wsConnsMu.Lock()
		wsIDGen++
		cID := wsIDGen
		wsConns[cID] = wsInst
		wsConnsMu.Unlock()

		go wsInst.readLoop()

		return in.createWSValue(cID, wsInst), nil
	}

	// ws_send(ws_handle, message)
	in.Builtins["ws_send"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("ws_send requires ws_handle and message arguments")
		}
		inst := in.getWSInstance(args[0])
		if inst == nil {
			return nil, fmt.Errorf("invalid websocket connection handle")
		}

		msg := args[1].String()
		if err := inst.writeFrame(1, []byte(msg)); err != nil {
			return nil, fmt.Errorf("failed sending websocket message: %w", err)
		}
		return BoolValue(true), nil
	}

	// ws_receive(ws_handle)
	in.Builtins["ws_receive"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("ws_receive requires ws_handle")
		}
		inst := in.getWSInstance(args[0])
		if inst == nil {
			return nil, fmt.Errorf("invalid websocket connection handle")
		}

		val, ok := <-inst.recvChan
		if !ok {
			return NilValue(), nil
		}
		return val, nil
	}

	// ws_on_message(ws_handle, callable)
	in.Builtins["ws_on_message"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("ws_on_message requires ws_handle and callable arguments")
		}
		inst := in.getWSInstance(args[0])
		if inst == nil {
			return nil, fmt.Errorf("invalid websocket connection handle")
		}
		inst.mu.Lock()
		inst.onMsg = args[1]
		inst.mu.Unlock()
		return BoolValue(true), nil
	}

	// ws_close(ws_handle)
	in.Builtins["ws_close"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		inst := in.getWSInstance(args[0])
		if inst == nil {
			return BoolValue(false), nil
		}
		inst.close()
		return BoolValue(true), nil
	}

	// ws_frame_encode(data, [is_client]) -> encoded frame string
	in.Builtins["ws_frame_encode"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("ws_frame_encode requires data argument")
		}
		data := []byte(args[0].String())
		isClient := false
		if len(args) >= 2 {
			isClient = args[1].IsTrue()
		}

		var header []byte
		header = append(header, 0x81) // FIN + text opcode

		length := len(data)
		if isClient {
			maskKey := make([]byte, 4)
			rand.Read(maskKey)

			if length <= 125 {
				header = append(header, 0x80|byte(length))
			} else if length <= 65535 {
				header = append(header, 0x80|126, byte(length>>8), byte(length))
			} else {
				header = append(header, 0x80|127)
				ext := make([]byte, 8)
				binary.BigEndian.PutUint64(ext, uint64(length))
				header = append(header, ext...)
			}
			header = append(header, maskKey...)

			masked := make([]byte, length)
			for i := 0; i < length; i++ {
				masked[i] = data[i] ^ maskKey[i%4]
			}
			return StringValue(string(append(header, masked...))), nil
		}

		if length <= 125 {
			header = append(header, byte(length))
		} else if length <= 65535 {
			header = append(header, 126, byte(length>>8), byte(length))
		} else {
			header = append(header, 127)
			ext := make([]byte, 8)
			binary.BigEndian.PutUint64(ext, uint64(length))
			header = append(header, ext...)
		}
		return StringValue(string(append(header, data...))), nil
	}
}

func (in *Interp) createWSValue(id uintptr, inst *wsConnInstance) *Value {
	res := make(map[string]*Value)
	res["handle"] = NativePtrValue(id)
	res["send"] = ClosureValue(&Closure{
		Name: "ws_send",
		Body: &BlockStmt{},
		Env:  in.GlobalEnv,
	})
	res["receive"] = ClosureValue(&Closure{
		Name: "ws_receive",
		Body: &BlockStmt{},
		Env:  in.GlobalEnv,
	})
	res["on_message"] = ClosureValue(&Closure{
		Name: "ws_on_message",
		Body: &BlockStmt{},
		Env:  in.GlobalEnv,
	})
	res["close"] = ClosureValue(&Closure{
		Name: "ws_close",
		Body: &BlockStmt{},
		Env:  in.GlobalEnv,
	})
	return HashValue(res)
}

func (in *Interp) getWSInstance(v *Value) *wsConnInstance {
	if v == nil {
		return nil
	}
	var id uintptr
	if v.Type == ValNativePtr {
		id = v.PtrVal
	} else if v.Type == ValHash && v.HashVal != nil {
		if h, ok := v.HashVal["handle"]; ok && h.Type == ValNativePtr {
			id = h.PtrVal
		}
	}
	wsConnsMu.Lock()
	inst := wsConns[id]
	wsConnsMu.Unlock()
	return inst
}

func (inst *wsConnInstance) readLoop() {
	for {
		header := make([]byte, 2)
		if _, err := io.ReadFull(inst.reader, header); err != nil {
			inst.close()
			break
		}

		opcode := header[0] & 0x0F
		masked := (header[1] & 0x80) != 0
		payloadLen := uint64(header[1] & 0x7F)

		if payloadLen == 126 {
			extLen := make([]byte, 2)
			io.ReadFull(inst.reader, extLen)
			payloadLen = uint64(binary.BigEndian.Uint16(extLen))
		} else if payloadLen == 127 {
			extLen := make([]byte, 8)
			io.ReadFull(inst.reader, extLen)
			payloadLen = binary.BigEndian.Uint64(extLen)
		}

		var maskKey []byte
		if masked {
			maskKey = make([]byte, 4)
			io.ReadFull(inst.reader, maskKey)
		}

		payload := make([]byte, payloadLen)
		io.ReadFull(inst.reader, payload)

		if masked {
			for i := uint64(0); i < payloadLen; i++ {
				payload[i] ^= maskKey[i%4]
			}
		}

		// Handle Close opcode
		if opcode == 0x08 {
			inst.close()
			break
		}

		// Text frame (opcode 1) or Binary frame (opcode 2)
		if opcode == 0x01 || opcode == 0x02 {
			msgVal := StringValue(string(payload))
			inst.mu.Lock()
			onMsg := inst.onMsg
			inst.mu.Unlock()

			if onMsg != nil {
				_, _ = inst.in.InvokeCallable(onMsg, []*Value{msgVal})
			}

			select {
			case inst.recvChan <- msgVal:
			default:
			}
		}
	}
}

func (inst *wsConnInstance) writeFrame(opcode byte, data []byte) error {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	if inst.closed {
		return fmt.Errorf("websocket connection is closed")
	}

	var header []byte
	header = append(header, 0x80|opcode) // FIN + opcode

	length := len(data)
	if inst.isClient {
		// Client frames MUST be masked according to RFC 6455
		maskKey := make([]byte, 4)
		rand.Read(maskKey)

		if length <= 125 {
			header = append(header, 0x80|byte(length))
		} else if length <= 65535 {
			header = append(header, 0x80|126)
			header = append(header, byte(length>>8), byte(length))
		} else {
			header = append(header, 0x80|127)
			ext := make([]byte, 8)
			binary.BigEndian.PutUint64(ext, uint64(length))
			header = append(header, ext...)
		}
		header = append(header, maskKey...)

		maskedData := make([]byte, length)
		for i := 0; i < length; i++ {
			maskedData[i] = data[i] ^ maskKey[i%4]
		}
		if _, err := inst.conn.Write(header); err != nil {
			return err
		}
		_, err := inst.conn.Write(maskedData)
		return err
	}

	// Server unmasked frame
	if length <= 125 {
		header = append(header, byte(length))
	} else if length <= 65535 {
		header = append(header, 126, byte(length>>8), byte(length))
	} else {
		header = append(header, 127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(length))
		header = append(header, ext...)
	}

	if _, err := inst.conn.Write(header); err != nil {
		return err
	}
	_, err := inst.conn.Write(data)
	return err
}

func (inst *wsConnInstance) close() {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	if !inst.closed {
		inst.closed = true
		_ = inst.conn.Close()
		close(inst.recvChan)
	}
}

func computeWSAcceptKey(key string) string {
	const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
