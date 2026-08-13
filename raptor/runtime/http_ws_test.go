package raptor

import (
	"testing"
	"time"
)

func TestHTTPServerAndClient(t *testing.T) {
	in := NewInterp()

	code := `
my $srv = http_server_listen(29880, sub ($req) {
    return {
        :status => 200,
        :body => "Raptor HTTP Response for " ~ $req<path>
    };
});

# Wait a brief moment for listener
sleep(0.05);

my $resp = http_get("http://127.0.0.1:29880/api/status");
http_server_close($srv);

[$resp<status>, $resp<body>];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("HTTP server/client eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected 2 elements, got %+v", val)
	}

	if val.ArrayVal[0].IntVal != 200 {
		t.Errorf("expected status 200, got %v", val.ArrayVal[0])
	}
	if val.ArrayVal[1].String() != "Raptor HTTP Response for /api/status" {
		t.Errorf("expected body 'Raptor HTTP Response for /api/status', got %q", val.ArrayVal[1].String())
	}
}

func TestHTTPParseAndFormat(t *testing.T) {
	in := NewInterp()

	code := `
my $rawReq = "GET /users?sort=asc HTTP/1.1\r\nHost: localhost\r\nUser-Agent: Raptor\r\n\r\nhello-body";
my $parsed = http_parse_request($rawReq);

my $formatted = http_format_response(201, {:Server => "Raptor/1.0"}, "Created Object");

[$parsed<method>, $parsed<path>, $parsed<headers><Host>, $parsed<body>, $formatted];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("HTTP parse/format eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 5 {
		t.Fatalf("expected 5 items, got %+v", val)
	}

	if val.ArrayVal[0].String() != "GET" {
		t.Errorf("expected GET, got %q", val.ArrayVal[0].String())
	}
	if val.ArrayVal[1].String() != "/users?sort=asc" {
		t.Errorf("expected /users?sort=asc, got %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].String() != "localhost" {
		t.Errorf("expected localhost, got %q", val.ArrayVal[2].String())
	}
	if val.ArrayVal[3].String() != "hello-body" {
		t.Errorf("expected hello-body, got %q", val.ArrayVal[3].String())
	}
}

func TestWebSocketFraming(t *testing.T) {
	in := NewInterp()

	code := `
my $encoded = ws_frame_encode("Hello WS", 0);
$encoded;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("WS frame encode eval failed: %v", err)
	}
	if val.Type != ValString || len(val.StrVal) == 0 {
		t.Fatalf("expected encoded frame string, got %+v", val)
	}
	_ = time.Second
}
