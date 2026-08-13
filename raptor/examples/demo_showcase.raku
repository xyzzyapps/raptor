# Raptor Comprehensive Feature Showcase
# Demonstrating: Non-OO procedural architecture, Sockets, HTTP, SQLite, JSON,
# PortAudio, Operators, Base Functions, Concurrency, and Raku sub MAIN.

sub MAIN() {
    say "=== Raptor Feature Showcase ===";

    # 1. Operators Suite
    say "\n[1] Operators Suite:";
    my $undef;
    my $defined = $undef // "Default Value";
    my $ternary = 10 > 5 ?? "10 is greater" !! "5 is greater";
    my $pow = 2 ** 8;
    my $bit = (0x0F +| 0xF0) +& 0xFF;
    my $rep = "Raptor " x 3;
    my $listRep = [10, 20] xx 2;
    my $isEven = 100 %% 2;
    my $m = 15 min 42;
    my $match = "Version 2.0" =~ "\\d+\\.\\d+";

    say "  Defined-or: " ~ $defined;
    say "  Ternary: " ~ $ternary;
    say "  Power 2**8: " ~ $pow;
    say "  Bitwise (0x0F +| 0xF0) +& 0xFF: " ~ $bit;
    say "  Repetition: " ~ $rep;
    say "  List rep count: " ~ $listRep.elems;
    say "  Divisibility 100 %% 2: " ~ $isEven;
    say "  Min (15 min 42): " ~ $m;
    say "  Regex match: " ~ $match;

    # 2. SQLite & JSON
    say "\n[2] SQLite & JSON Support:";
    my $db = sqlite_open(":memory:");
    sqlite_exec($db, "CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, price REAL);");
    sqlite_exec($db, "INSERT INTO items (name, price) VALUES ('Quantum Core', 499.99);");
    sqlite_exec($db, "INSERT INTO items (name, price) VALUES ('Plasma Relay', 129.50);");

    my $rows = sqlite_query($db, "SELECT * FROM items ORDER BY id ASC;");
    sqlite_close($db);

    my $jsonData = to_json({:count => $rows.elems, :items => $rows});
    say "  JSON payload from SQLite: " ~ $jsonData;

    # 3. TCP Sockets & HTTP
    say "\n[3] TCP Sockets & HTTP:";
    my $srv = tcp_listen(29899);
    my $cli = tcp_connect("127.0.0.1", 29899);
    my $acc = tcp_accept($srv);

    tcp_send($cli, "GET /api/health HTTP/1.1\r\nHost: localhost\r\n\r\n");
    my $reqRaw = tcp_recv($acc<socket>);
    my $parsedReq = http_parse_request($reqRaw);

    my $respBody = to_json({:status => "ok", :path => $parsedReq<path>});
    my $httpResp = http_format_response(200, {:Content-Type => "application/json"}, $respBody);
    tcp_send($acc<socket>, $httpResp);

    my $clientRecv = tcp_recv($cli);
    tcp_close($cli);
    tcp_close($acc<socket>);
    tcp_close($srv);
    say "  HTTP over TCP response: " ~ substr($clientRecv, 0, 50) ~ "...";

    # 4. PortAudio Engine
    say "\n[4] PortAudio Audio Engine:";
    pa_init();
    my $paVer = pa_get_version_text();
    my $devCount = pa_device_count();
    my $sineWave = pa_generate_sine_wave(440.0, 0.05, 44100.0, 0.5);
    pa_terminate();
    say "  PortAudio Version: " ~ $paVer;
    say "  Audio Devices Found: " ~ $devCount;
    say "  Synthesized Sine Wave Samples: " ~ $sineWave.elems;

    # 5. Advanced Concurrency
    say "\n[5] Advanced Concurrency:";
    my $inputs = [1, 2, 3, 4, 5, 6];
    my $squares = parallel_map($inputs, sub ($x, $idx) {
        return $x * $x;
    }, 4);
    say "  Parallel Map (x*x): [" ~ join(", ", $squares) ~ "]";

    my $sup = supply_create();
    my $total = 0;
    supply_tap($sup, sub ($val) {
        $total = $total + $val;
    });
    supply_emit($sup, 100);
    supply_emit($sup, 200);
    supply_done($sup);
    say "  Reactive Supply Sum: " ~ $total;

    # 6. Environment & System Globals
    say "\n[6] Environment & Globals:";
    say "  PID: " ~ $*PID;
    say "  OS: " ~ $*OS;
    say "  Executable: " ~ $*EXECUTABLE;
    say "  Program Name: " ~ $*PROGRAM-NAME;
    say "  Time (epoch): " ~ time();

    say "\n=== All Demonstrations Completed Successfully ===";
    return 0;
}
