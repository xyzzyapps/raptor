package raptor

import (
	"testing"
	"time"
)

func TestTCPSocketsClientServer(t *testing.T) {
	in := NewInterp()

	serverCode := `
my $srv = tcp_listen(29876);
my $client = tcp_connect("127.0.0.1", 29876);
my $acc = tcp_accept($srv);
my $accSock = $acc<socket>;

my $sent = tcp_send($client, "Hello Raptor TCP!");
my $received = tcp_recv($accSock);

my $replySent = tcp_send($accSock, "ACK: " ~ $received);
my $replyRecv = tcp_recv($client);

tcp_close($client);
tcp_close($accSock);
tcp_close($srv);

[$sent, $received, $replyRecv];
`

	val, err := in.Eval(serverCode)
	if err != nil {
		t.Fatalf("TCP eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected 3 results, got %+v", val)
	}

	if val.ArrayVal[0].IntVal != int64(len("Hello Raptor TCP!")) {
		t.Errorf("expected sent len 17, got %v", val.ArrayVal[0])
	}
	if val.ArrayVal[1].String() != "Hello Raptor TCP!" {
		t.Errorf("expected 'Hello Raptor TCP!', got %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].String() != "ACK: Hello Raptor TCP!" {
		t.Errorf("expected 'ACK: Hello Raptor TCP!', got %q", val.ArrayVal[2].String())
	}
}

func TestUDPSocketsSendRecv(t *testing.T) {
	in := NewInterp()

	code := `
my $s1 = udp_bind(29877);
my $s2 = udp_bind(29878);

my $sent = udp_sendto($s1, "127.0.0.1", 29878, "Ping UDP Datagram");
my $recv = udp_recvfrom($s2);

udp_close($s1);
udp_close($s2);

[$sent, $recv<data>];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("UDP eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected 2 results, got %+v", val)
	}

	if val.ArrayVal[1].String() != "Ping UDP Datagram" {
		t.Errorf("expected 'Ping UDP Datagram', got %q", val.ArrayVal[1].String())
	}
}

func TestSocketTimeout(t *testing.T) {
	in := NewInterp()

	code := `
my $srv = tcp_listen(29879);
my $client = tcp_connect("127.0.0.1", 29879);
socket_set_timeout($client, 100);
my $acc = tcp_accept($srv);

tcp_close($client);
tcp_close($acc<socket>);
tcp_close($srv);
1;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("socket timeout eval failed: %v", err)
	}
	if !val.IsTrue() {
		t.Errorf("expected true, got %v", val)
	}
	_ = time.Second
}
