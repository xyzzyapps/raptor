//go:build !js || !wasm

package raptor

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type socketManager struct {
	mu          sync.Mutex
	tcpListener map[string]net.Listener
	tcpConn     map[string]net.Conn
	udpConn     map[string]*net.UDPConn
	nextID      int
}

var globalSockets = &socketManager{
	tcpListener: make(map[string]net.Listener),
	tcpConn:     make(map[string]net.Conn),
	udpConn:     make(map[string]*net.UDPConn),
	nextID:      1,
}

func (in *Interp) registerSocketBuiltins() {
	// tcp_listen(port_or_addr) -> listener handle
	in.Builtins["tcp_listen"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("tcp_listen requires port or address argument")
		}
		addr := args[0].String()
		if !strings.Contains(addr, ":") {
			addr = ":" + addr
		}

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("tcp_listen failed on %s: %w", addr, err)
		}

		globalSockets.mu.Lock()
		key := fmt.Sprintf("tcpl_%d", globalSockets.nextID)
		globalSockets.nextID++
		globalSockets.tcpListener[key] = listener
		globalSockets.mu.Unlock()

		return StringValue(key), nil
	}

	// tcp_accept(listener_handle, [timeout_ms]) -> { socket, addr, port }
	in.Builtins["tcp_accept"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("tcp_accept requires listener handle")
		}
		key := args[0].String()

		globalSockets.mu.Lock()
		listener, ok := globalSockets.tcpListener[key]
		globalSockets.mu.Unlock()

		if !ok || listener == nil {
			return nil, fmt.Errorf("invalid or closed tcp listener handle %q", key)
		}

		conn, err := listener.Accept()
		if err != nil {
			return nil, fmt.Errorf("tcp_accept error: %w", err)
		}

		globalSockets.mu.Lock()
		cKey := fmt.Sprintf("tcps_%d", globalSockets.nextID)
		globalSockets.nextID++
		globalSockets.tcpConn[cKey] = conn
		globalSockets.mu.Unlock()

		res := make(map[string]*Value)
		res["socket"] = StringValue(cKey)
		res["handle"] = StringValue(cKey)
		if rAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			res["addr"] = StringValue(rAddr.IP.String())
			res["port"] = IntValue(int64(rAddr.Port))
		} else {
			res["addr"] = StringValue(conn.RemoteAddr().String())
			res["port"] = IntValue(0)
		}

		return HashValue(res), nil
	}

	// tcp_connect(host, port, [timeout_ms]) -> socket handle
	in.Builtins["tcp_connect"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("tcp_connect requires host and port arguments")
		}
		host := args[0].String()
		port := args[1].String()
		addr := net.JoinHostPort(host, port)

		timeout := 10 * time.Second
		if len(args) >= 3 && args[2].Type != ValNil {
			ms := in.toInt(args[2])
			if ms > 0 {
				timeout = time.Duration(ms) * time.Millisecond
			}
		}

		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return nil, fmt.Errorf("tcp_connect to %s failed: %w", addr, err)
		}

		globalSockets.mu.Lock()
		cKey := fmt.Sprintf("tcps_%d", globalSockets.nextID)
		globalSockets.nextID++
		globalSockets.tcpConn[cKey] = conn
		globalSockets.mu.Unlock()

		return StringValue(cKey), nil
	}

	// tcp_send(sock_handle, data) -> bytes sent
	in.Builtins["tcp_send"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("tcp_send requires socket handle and data arguments")
		}
		key := args[0].String()

		globalSockets.mu.Lock()
		conn, ok := globalSockets.tcpConn[key]
		globalSockets.mu.Unlock()

		if !ok || conn == nil {
			return nil, fmt.Errorf("invalid or closed tcp socket handle %q", key)
		}

		data := args[1].String()
		n, err := conn.Write([]byte(data))
		if err != nil {
			return nil, fmt.Errorf("tcp_send failed: %w", err)
		}
		return IntValue(int64(n)), nil
	}

	// tcp_recv(sock_handle, [max_bytes]) -> string
	in.Builtins["tcp_recv"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("tcp_recv requires socket handle argument")
		}
		key := args[0].String()

		globalSockets.mu.Lock()
		conn, ok := globalSockets.tcpConn[key]
		globalSockets.mu.Unlock()

		if !ok || conn == nil {
			return nil, fmt.Errorf("invalid or closed tcp socket handle %q", key)
		}

		maxBytes := 4096
		if len(args) >= 2 && args[1].Type != ValNil {
			mb := int(in.toInt(args[1]))
			if mb > 0 {
				maxBytes = mb
			}
		}

		buf := make([]byte, maxBytes)
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				if n > 0 {
					return StringValue(string(buf[:n])), nil
				}
				return StringValue(""), nil
			}
			return nil, fmt.Errorf("tcp_recv failed: %w", err)
		}
		return StringValue(string(buf[:n])), nil
	}

	// tcp_close(handle) -> bool
	in.Builtins["tcp_close"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		key := args[0].String()

		globalSockets.mu.Lock()
		if listener, ok := globalSockets.tcpListener[key]; ok {
			delete(globalSockets.tcpListener, key)
			globalSockets.mu.Unlock()
			_ = listener.Close()
			return BoolValue(true), nil
		}
		if conn, ok := globalSockets.tcpConn[key]; ok {
			delete(globalSockets.tcpConn, key)
			globalSockets.mu.Unlock()
			_ = conn.Close()
			return BoolValue(true), nil
		}
		globalSockets.mu.Unlock()

		return BoolValue(false), nil
	}

	// udp_bind(port_or_addr) -> UDP socket handle
	in.Builtins["udp_bind"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("udp_bind requires port or address argument")
		}
		addrStr := args[0].String()
		if !strings.Contains(addrStr, ":") {
			addrStr = ":" + addrStr
		}

		lAddr, err := net.ResolveUDPAddr("udp", addrStr)
		if err != nil {
			return nil, fmt.Errorf("failed resolving udp address %s: %w", addrStr, err)
		}

		conn, err := net.ListenUDP("udp", lAddr)
		if err != nil {
			return nil, fmt.Errorf("udp_bind failed on %s: %w", addrStr, err)
		}

		globalSockets.mu.Lock()
		key := fmt.Sprintf("udps_%d", globalSockets.nextID)
		globalSockets.nextID++
		globalSockets.udpConn[key] = conn
		globalSockets.mu.Unlock()

		return StringValue(key), nil
	}

	// udp_sendto(sock_handle, host, port, data) -> bytes sent
	in.Builtins["udp_sendto"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 4 {
			return nil, fmt.Errorf("udp_sendto requires sock_handle, host, port, data arguments")
		}
		key := args[0].String()
		host := args[1].String()
		port := int(in.toInt(args[2]))
		data := args[3].String()

		globalSockets.mu.Lock()
		conn, ok := globalSockets.udpConn[key]
		globalSockets.mu.Unlock()

		if !ok || conn == nil {
			return nil, fmt.Errorf("invalid or closed udp socket handle %q", key)
		}

		destAddr := &net.UDPAddr{
			IP:   net.ParseIP(host),
			Port: port,
		}
		if destAddr.IP == nil {
			rAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
			if err != nil {
				return nil, fmt.Errorf("failed resolving UDP destination %s:%d: %w", host, port, err)
			}
			destAddr = rAddr
		}

		n, err := conn.WriteToUDP([]byte(data), destAddr)
		if err != nil {
			return nil, fmt.Errorf("udp_sendto failed: %w", err)
		}
		return IntValue(int64(n)), nil
	}

	// udp_recvfrom(sock_handle, [max_bytes]) -> { data, addr, port }
	in.Builtins["udp_recvfrom"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("udp_recvfrom requires socket handle argument")
		}
		key := args[0].String()

		globalSockets.mu.Lock()
		conn, ok := globalSockets.udpConn[key]
		globalSockets.mu.Unlock()

		if !ok || conn == nil {
			return nil, fmt.Errorf("invalid or closed udp socket handle %q", key)
		}

		maxBytes := 4096
		if len(args) >= 2 && args[1].Type != ValNil {
			mb := int(in.toInt(args[1]))
			if mb > 0 {
				maxBytes = mb
			}
		}

		buf := make([]byte, maxBytes)
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return nil, fmt.Errorf("udp_recvfrom failed: %w", err)
		}

		res := make(map[string]*Value)
		res["data"] = StringValue(string(buf[:n]))
		if remoteAddr != nil {
			res["addr"] = StringValue(remoteAddr.IP.String())
			res["port"] = IntValue(int64(remoteAddr.Port))
		} else {
			res["addr"] = StringValue("")
			res["port"] = IntValue(0)
		}
		return HashValue(res), nil
	}

	// udp_close(sock_handle) -> bool
	in.Builtins["udp_close"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		key := args[0].String()

		globalSockets.mu.Lock()
		conn, ok := globalSockets.udpConn[key]
		if ok {
			delete(globalSockets.udpConn, key)
		}
		globalSockets.mu.Unlock()

		if !ok || conn == nil {
			return BoolValue(false), nil
		}

		_ = conn.Close()
		return BoolValue(true), nil
	}

	// socket_set_timeout(sock_handle, timeout_ms)
	in.Builtins["socket_set_timeout"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("socket_set_timeout requires sock_handle and timeout_ms arguments")
		}
		key := args[0].String()
		ms := in.toInt(args[1])
		var deadline time.Time
		if ms > 0 {
			deadline = time.Now().Add(time.Duration(ms) * time.Millisecond)
		}

		globalSockets.mu.Lock()
		defer globalSockets.mu.Unlock()

		if conn, ok := globalSockets.tcpConn[key]; ok {
			_ = conn.SetDeadline(deadline)
			return BoolValue(true), nil
		}
		if conn, ok := globalSockets.udpConn[key]; ok {
			_ = conn.SetDeadline(deadline)
			return BoolValue(true), nil
		}

		return BoolValue(false), nil
	}
}
