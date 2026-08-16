<p align="center">
  <img src="assets/logo_wt.png" alt="Raptor Language Logo" width="160" />
</p>

# Raptor

A **Perl 5–shaped** dynamic language: the non-OO subset of Raku. No `class`, `has`, or inheritance. Records are C structs. Types are optional `subset` refinements checked on every assignment.

**Parse:** [gcre](../gcre) loads [`runtime/raptor.raku`](runtime/raptor.raku). Pratt is only `<HOST_stmt>` / `<HOST_expr>` inside that grammar — not a second full-file parser.

**Backends:** `--go` (default tree-walk), `--moar` (CompUnit v7 on `moar.dll`, same opcode family as Tcl in [../moarvm-go](../moarvm-go)), `--wasm` (TinyGo if available, else `GOOS=js`; `--wasm-compiler=go|tinygo`).

**Docs:** PodLit in `docs/` — `raptor doc perlraptor`, `raptor weave docs/index.pod`, numbered `01_*.md` chapters. `raptor pack` embeds gcre + the runtime (`replace gcre => ../gcre`).

If you know Perl 5, you already know Raptor.

| Perl 5 | Raptor |
| :--- | :--- |
| `my $x`, `my @a`, `my %h` | same |
| `$_` topic | same (`for`, `given`, bare `say`) |
| `.` concat | `~` |
| `x` / `//` / `eq ne lt gt` | same (`//=` too; also `==` numeric) |
| `$obj->method` | `$obj.method()` **UFCS** (any `sub method($obj, …)`) |
| `bless` / `@ISA` | no classes — `struct` + `multi sub` |
| `package Foo;` | same; `%Foo::` stash |
| TAP `ok` / `is` | built in (`raptor test t/`) |
| XS | `is native('lib.dll')` NativeCall |

```perl
my $name = "Raptor";
my @nums = [1, 2, 3];
my %cfg  = { "port" => 8080 };
$_ = $name;
say;                          # topic
say 2 ** 10;                  # 1024
say 6 × 7;                    # unicode ops
say $cfg{"port"} // 80;

struct Point { num64 $x; num64 $y; }
subset Positive where { $_ > 0 };
my Positive $n = 10;
```

Licensed under the **Artistic License 2.0**.

---

## Feature catalog

1. **Sigils, `Nil`, operators** — `$scalar`, `@array`, `%hash`; defined-or (`//`, `//=`), `**`, ternary (`?? !!` / `?:`), bitwise (`+&` `+|` `+^` `+<` `+>`), `x` / `xx`, `div` / `mod` / `%%`, `min` / `max`, file tests (`-e -f -d -s -r -w`), `=~` / `!~`, unicode `× ÷ √ ∑ ∏`.
2. **Topic `$_`** — `for`, `given` / `when`, bare `say`, statement modifiers.
3. **UFCS** — `$val.func()`, `20.double()`, `"raptor".uc()`, `@list.map(...)`.
4. **`subset` + predicate dispatch** — `subset Positive where { $_ > 0 }`; `multi sub fib($n where { $n <= 1 })`; invariants on every assignment.
5. **C-ABI `struct` + closures** — `struct Vector2 { num64 $x; num64 $y; }`; `$btn.onClick = sub ($v) { ... }`; `multi sub infix:<+>(Vec2 $a, Vec2 $b)`.
6. **Junctions** — `any` / `all` / `one` / `none` autothreading.
7. **`gather` / `take`** — lazy sequences.
8. **Statement modifiers & heredocs** — postfix `if` / `unless` / `while` / `until` / `for` / `given`; `<<EOF` / `<<~EOF`.
9. **References** — `\$s`, `\@a`, `\%h`, `\&sub`; `$$s`, `$a->[0]`, `$h->{"k"}`, `ref()`, `is_ref()`.
10. **Labels & goto** — `LABEL:`, `goto LABEL;`, `goto &target_sub;`.
11. **Scoping** — `my`, `our`, `state`.
12. **Packages & AUTOLOAD** — `package Foo;`, `%Foo::`, `package_symbols` / `package_set` / `package_get` / `package_delete`; `$AUTOLOAD`.
13. **Contextual vars** — `@*ARGS`, `%*ENV`, `$*PROGRAM`, `$*RAPTOR`, `$*KERNEL`, `$*PID`, `$?`, `$!`, `$$`, `$0`.
14. **Backticks** — `` `cmd` ``, `qx{cmd}`; `$?` / `$!`.
15. **Grammars** — `grammar` / `rule` / `token` / `regex` (gcre). Language parse is gcre + HOST Pratt.
16. **Regex / `samre`** — `regex_engine("samre")`.
17. **Verification** — `pre` / `post` / `invariant`; `TEST`, `PRE`, `POST`, `INVARIANT`, `PROPERTY`, `SUBTEST`, `CHECK`, `ASSERT`; TAP `plan` / `ok` / `is` / `is_deeply` / `like` / `done_testing`; `raptor test t/` (like `prove`).
18. **TUI (Charmbracelet)** — `tui_style`, `tui_box`, `tui_table`, `tui_progress`, `tui_markdown`, `tui_app_run`.
19. **PortAudio** — `pa_init`, `pa_device_count`, `pa_sine_wave`, … (`lib/PortAudio.rp`).
20. **SQLite & JSON** — `sqlite_open` / `exec` / `query` / `close`; `to_json` / `from_json`.
21. **Sockets** — `tcp_listen` / `accept` / `connect` / `send` / `recv`; `udp_bind` / `send` / `recv`.
22. **HTTP/1.1 & WebSockets** — `http_get`, `http_post`, `http_server_start`, `http_format_response`; `ws_frame_text`, `ws_parse_frame`.
23. **Raylib 5.5** — NativeCall (`lib/Raylib.rp`); [examples/raylib_game.rp](examples/raylib_game.rp).
24. **Concurrency** — `start` / `await`, `Channel`, `Mutex`, `Semaphore`, `WaitGroup`, `parallel_map`, `Supply`, `atomic_add` / `cas`.
25. **Advice** — `before` / `after` / `around` on multi-subs.
26. **NativeCall / FFI** — `is native('lib.dll')`; `ffi_load` / `ffi_call`.
27. **GGML tensor C API** — `ggml_init`, `ggml_new_tensor_2d`, `ggml_mul_mat` (`A^T * B`), `ggml_relu` / `gelu` / `soft_max`, `ggml_graph_compute_with_ctx` (software; optional `ggml.dll`). [lib/GGML.rp](lib/GGML.rp), [examples/ggml_tensors.rp](examples/ggml_tensors.rp).
28. **Tiny char LM** — `llm_tiny_load` / `generate` / `logits` (CPU; WebGPU matmul when the adapter is ready). Tour lesson currently off.
29. **Package manager** — `raptor init`, `raptor get <repo>`, `raptor install` → `./raptor_modules/`.
30. **`raptor pack`** — script + runtime into one `.exe` (same interpreter, not a speed win).
31. **PodLit** — `raptor weave` / `tangle` / `stitch`; run `.pod` directly; `raptor doc`.
32. **RaptorHP** — PHP-style `<?raptor ?>` / `<?= ?>` in `.phtml`. `raptor -S localhost:8000` (or `raptorhp -S`) like `php -S`; `-t` docroot; optional router. `%_GET` / `%_POST` / `%_SERVER`.
33. **WASM tour** — `raptor serve` → http://localhost:8080/. Canvas 2D, WebGL, WebAudio **node graph** (osc / filter / compressor / delay / LFO on `AudioContext.currentTime`). WebGPU and HTTP tour pages are disabled for now. `raptor --wasm` prefers TinyGo; writes `raptor_bridge.js` + `wasm_exec.js` next to the `.wasm`.
34. **Embedded / TinyGo** — ESP32 / RP2040 profile: `gpio_*`, `pwm_*`, `i2c_*`, `spi_transfer`, UART REPL. [examples/esp32_blink.rp](examples/esp32_blink.rp).
35. **Memory (interpreter)** — interned ±256 ints, in-place `+=` / `$s = $s ~ x`, recycled loop `Env`s, one-shot HOST lex, int64 array lane. See [SPEC.md](SPEC.md) §8.

---

## Language sketches

**UFCS**
```perl
sub format_id($n) { return "ID-#" ~ $n; }
say 1001.format_id();
say "welcome".uc();
```

**Predicates**
```perl
subset PortNum where { 1 <= $_ <= 65535 };
my PortNum $port = 8080;
multi sub fib($n where { $n <= 1 }) { return $n; }
multi sub fib($n) { return fib($n - 1) + fib($n - 2); }
```

**Structs**
```perl
struct Vector2 { num64 $x; num64 $y; }
multi sub infix:<+>(Vector2 $a, Vector2 $b) {
    my $r = Vector2.new();
    $r.x = $a.x + $b.x;
    $r.y = $a.y + $b.y;
    return $r;
}
```

**Contracts & TAP**
```perl
sub safe_divide($a, $b) {
    pre({ $b != 0 });
    post({ $_ >= 0 });
    return $a div $b;
}
property "addition commutativity", sub ($x, $y) { return ($x + $y) == ($y + $x); };
plan(1); ok(1 + 1 == 2, "math"); done_testing();
```

**TUI**
```perl
say tui_style("RAPTOR", {:fg => "#00ADD8", :bold => True});
say tui_box("ok", {:title => "status", :border => "rounded"});
```

**WebAudio (WASM)** — graph in Raptor, timeline on `currentTime` (C4–E4–G4–B4–C5 in the tour):
```perl
my $ctx = audio_context_create();
my $t0 = audio_get_current_time($ctx);
my $osc = audio_create_oscillator($ctx);
my $env = audio_create_gain($ctx);
audio_set_frequency($osc, 261.63, $t0);
audio_connect($osc, $env);
audio_connect_destination($env, $ctx);
audio_osc_start($osc, $t0);
```

---

## Binaries

| Binary | Source | Description |
| :--- | :--- | :--- |
| **`bin/raptor`** | `cmd/raptor/main.go` | Run, REPL, `--go` / `--moar` / `--wasm`, `test`, `doc`, weave/tangle/stitch, `init`/`get`/`install`, `serve`, `pack`, **`-S`** (RaptorHP) |
| **`bin/raptorhp`** | `cmd/raptorhp/main.go` | Templates: `raptorhp file.phtml`, `-r '…'`, `-S localhost:8000 -t public` |
| **`web/raptor.wasm`** | `cmd/wasm/main.go` | Browser engine (`raptorEval`, weave/tangle/stitch) |

`raptor pack script.rp -o app.exe` embeds the script + runtime. Bundled DLLs in `bin/` (not built here): `moar.dll`, `libraylib.dll`, `sqlite3.dll`. `ffi_load` searches `bin/`, the exe dir, and `PATH`.

---

## Building & testing

**Need:** Go 1.22+, `replace gcre => ../gcre` and `replace moarvm-go => ../moarvm-go`. Use `-mod=mod` (do not `go mod vendor` — `vendor/` under moarvm-go is MoarVM C).

```powershell
cd raptor
go build -mod=mod -o bin/raptor.exe ./cmd/raptor
go build -mod=mod -o bin/raptorhp.exe ./cmd/raptorhp
go test -mod=mod ./...
.\bin\raptor.exe test t\
.\bin\raptor.exe run examples/demo_showcase.rp
.\bin\raptor.exe serve --port 8080
.\bin\raptor.exe -S localhost:8000
.\bin\raptor.exe --wasm --wasm-compiler=go -o web/raptor.wasm
.\bin\raptor.exe pack examples/raylib_game.rp -o bin/raylib_game.exe
.\bin\raptor.exe doc perlraptor
```

Linux: `go build -o bin/raptor ./cmd/raptor` and the same `test` / `serve` / `-S` commands. Packages: `gcc`, `libsqlite3-dev`, `portaudio` as needed.

Rakupp-style kernels: `examples/bench/*.rp`. Memory/time table: [SPEC.md](SPEC.md) §8.

---

## WASM tour & embedded

`raptor serve` hosts `web/` (Canvas, WebGL, WebAudio node graph). Tour HTTP and WebGPU lessons are off for now. Live Pages: [xyzzyapps.github.io/raptor](https://xyzzyapps.github.io/raptor/).

TinyGo / ESP32: `scripts/build_optimized.ps1`; `tinygo flash -target=esp32 ./cmd/esp32`. Host-sim: `examples/esp32_blink.rp`.

| Profile | Notes |
| :--- | :--- |
| Desktop CLI | `go build -ldflags="-s -w"` ≈ 8 MB stripped |
| WASM (`gc`) | `GOOS=js` playground (TinyGo smaller if `wasm-opt` is new enough) |
| ESP32 | TinyGo, ~380 KB flash |

---

## Notes

The first prototype was written by **Gemini**. The language was mostly written by Anti Gravity — Gemini 3.6. Library bindings and PodLit were written by Gemini 3.7. Later work was modified by **Grok**.

Built on Windows. It should work on Linux. Expect bugs, memory leaks, and some wrong semantics. Future versions will sit closer to Raku.
