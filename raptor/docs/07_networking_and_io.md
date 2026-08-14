# Raptor Networking, Sockets, SQLite & JSON

Raptor provides native procedural networking (TCP & UDP), HTTP/1.1 client/server, RFC 6455 WebSockets, embedded SQLite databases, and fast JSON encoding.

## 1. TCP & UDP Sockets

```perl
# TCP Server
my $server = tcp_listen(9000);
my $client = tcp_accept($server);
my $msg = tcp_recv($client{"socket"}, 1024);
tcp_send($client{"socket"}, "Echo: " ~ $msg);
tcp_close($client{"socket"});
tcp_close($server);

# TCP Client
my $sock = tcp_connect("127.0.0.1", 9000);
tcp_send($sock, "Hello Server");
my $response = tcp_recv($sock, 1024);
tcp_close($sock);
```

## 2. HTTP & WebSockets

```perl
# HTTP Client
my $res = http_get("https://api.github.com", { "User-Agent" => "Raptor/1.0" });
say "Status: " ~ $res{"status"};
say "Body: " ~ $res{"body"};

# RFC 6455 WebSocket Framing
my $frame = ws_frame_text("Hello WebSocket");
```

## 3. SQLite Embedded Database

```perl
# Open database
my $db = sqlite_open("app.db");

# Execute DDL
sqlite_exec($db, "CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT, role TEXT)");

# Insert records
sqlite_exec($db, "INSERT INTO users (name, role) VALUES ('Alice', 'Admin')");

# Query records (returns array of hashes)
my @rows = sqlite_query($db, "SELECT * FROM users");
for @rows -> $row {
    say "User: " ~ $row{"name"} ~ " (Role: " ~ $row{"role"} ~ ")";
}

sqlite_close($db);
```

## 4. JSON Serialization

```perl
# Convert native Raptor structures to JSON
my %data = { "service" => "Gateway", "ports" => [80, 443], "active" => True };
my $json_str = to_json(%data, True); # Pretty printed
say $json_str;

# Parse JSON into native hashes and arrays
my $parsed = from_json($json_str);
say "Parsed Service: " ~ $parsed{"service"};
```
