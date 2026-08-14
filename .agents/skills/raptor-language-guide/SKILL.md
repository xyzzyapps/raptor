---
name: raptor-language-guide
description: Comprehensive architecture, syntax, FFI, WebAssembly, RaptorHP templates, PodLit literate programming, and verification guide for AI agents working with the Raptor language runtime.
---

# Raptor Language Runtime Guide for AI Agents

## 1. Core Language Identity & Philosophy

Raptor is a high-performance, strictly non-OO procedural execution platform and language runtime designed as the **pure dynamic Perl 5 subset of Raku**.
- **No OOP Overhead**: No `class`, `has`, `is` keywords, or inheritance hierarchies.
- **Pure Dynamic Typing**: Dynamic variables (`$scalar`, `@array`, `%hash`), first-class subroutines, and C-ABI structs.
- **Dynamic Refinements (`subset`)**: Refine dynamic types using `where` boolean predicates without static type annotations.
- **File Extensions**: Primary extensions are `.rp` and `.raptor` (with `.pod` for literate documents and `.phtml` for templates).

---

## 2. Syntax & Data Structures

### 2.1 Variables & Sigils

```perl
my $scalar = "Dynamic String";
my @array  = [10, 20, 30, 40];
my %hash   = {:name => "Ada", :role => "Admin"};
```

### 2.2 C-ABI Structs & Function Pointers

Records have natural C memory layouts with support for first-class function pointer fields:

```perl
struct Vector2 {
    num64 $x;
    num64 $y;
}

# Overload operators on structs via multi sub
multi sub infix:<+>(Vector2 $a, Vector2 $b) {
    my $res = Vector2.new();
    $res.x = $a.x + $b.x;
    $res.y = $a.y + $b.y;
    return $res;
}

# Struct closures / method syntax
struct Button {
    Str $label;
    Any $onClick;
}

my $btn = Button.new();
$btn.label = "Submit";
$btn.onClick = sub ($val) { say "Clicked with: $val"; };
$btn.onClick(42); # Invokes closure via method syntax
```

### 2.3 Subsets & Predicate Multiple Dispatch

```perl
subset Positive of Int where { $_ > 0 };
subset Even of Int where { $_ % 2 == 0 };

multi sub classify(Even $n) { say "$n is Even"; }
multi sub classify(Positive $n) { say "$n is Positive"; }

# Predicate Pattern Matching in Function Signatures
multi sub fib(Int $n where { $n <= 1 }) { return $n; }
multi sub fib(Int $n) { return fib($n - 1) + fib($n - 2); }
```

### 2.4 Quantum Autothreading Junctions

```perl
my $target = 25;
if $target == any(10, 20, 25, 30) {
    say "Target matched in junction!";
}

my @scores = [85, 92, 78];
if all(@scores) > 70 {
    say "All scores passed threshold!";
}
```

---

## 3. Foreign Function Interface (NativeCall) & Raylib Graphics

### 3.1 C FFI Calling Conventions

```perl
# Load Windows API or C library
sub GetSystemTime(OpaquePointer $lpSystemTime) is native("kernel32.dll") { * }

# Call hardware Raylib graphics engine (60 FPS GUI)
use lib::Raylib;
InitWindow(800, 450, "Raptor Raylib Window");
SetTargetFPS(60);

while !WindowShouldClose() {
    BeginDrawing();
    ClearBackground(RAYWHITE);
    DrawText("Hello from Raptor 60 FPS Raylib!", 190, 200, 20, LIGHTGRAY);
    EndDrawing();
}
CloseWindow();
```

---

## 4. WebAssembly (Wasm) Browser Runtime

Raptor compiles to pure WebAssembly (`web/raptor.wasm`) running 100% client-side in browsers:
```powershell
raptor serve --port 8080
# Opens in-browser REPL and playground at http://localhost:8080/
```

### Web Built-ins:
- **Canvas 2D**: `canvas_init`, `canvas_clear`, `canvas_draw_rect`, `canvas_draw_circle`, `canvas_draw_line`, `canvas_draw_text`.
- **DOM Engine**: `dom_get`, `dom_set_text`, `dom_set_html`, `dom_create`.
- **WebAudio API**: `audio_init`, `audio_play_tone(freq, dur, wave)`, `audio_play_melody(@freqs, @durs)`.
- **JSON Serialization**: `to_json(%map)`, `from_json($str)`.

---

## 5. RaptorHP Embedded Template Server (`raptorhp.exe`)

Render PHP-style templates or launch a development web server:

```html
<!-- index.phtml -->
<h1><?= "Welcome to RaptorHP" ?></h1>
<ul>
<?rp for 1..5 -> $i { ?>
    <li>Item <?= $i * 10 ?> (Client: <?= %_SERVER{"REMOTE_ADDR"} ?>)</li>
<?rp } ?>
</ul>
```

### Commands:
```powershell
# Render template to stdout
raptorhp index.phtml

# Evaluate inline template expression
raptorhp -r '<h1><?= "Hello " ~ "World" ?></h1>'

# Start development HTTP server
raptorhp -S localhost:8000
```

---

## 6. PodLit Literate Programming Subsystem

Knuth-style literate programming combining human prose with executable code chunks:

```pod
=pod
=head1 Mathematical Engine

=chunk add-func :file(lib/Math/Add.rp)
sub add_numbers($a, $b) {
    return $a + $b;
}
=end chunk

=chunk main-calc :file(bin/calc.rp)
<<add-func>>
say "Sum: ", add_numbers(15, 30);
=end chunk
=cut
```

### Commands:
```powershell
# Weave human-readable Markdown documentation
raptor weave doc.pod -o doc.md

# Tangle executable source code files
raptor tangle doc.pod -o src/

# Stitch modified code back into POD document (round-trip)
raptor stitch doc.pod src/lib/Math/Add.rp

# Execute POD directly in-memory
raptor doc.pod
```

---

## 7. TAP Testing & Formal Verification

### 7.1 Test Anything Protocol (TAP)

```perl
use Test::More;
plan(4);

ok(1 + 1 == 2, "basic addition");
is(2 ** 10, 1024, "exponentiation");
is_deeply([1, 2, 3], [1, 2, 3], "array equality");
like("Raptor 1.0", '/Raptor/', "regex pattern match");

done_testing();
```

Run test suite via test harness:
```powershell
raptor test t/
```

### 7.2 Verification & Design-by-Contract

```perl
# Design-by-contract pre/post conditions
sub safe_divide(num64 $a, num64 $b)
    pre  { $b != 0 }
    post { $_ * $b == $a }
{
    return $a / $b;
}

# QuickCheck randomized property-based fuzzing
property "addition commutativity", sub (Int $a, Int $b) {
    return $a + $b == $b + $a;
};
```

---

## 8. CLI Command Summary

| Command | Action |
| :--- | :--- |
| `raptor run <script.rp>` | Execute Raptor script |
| `raptor init [name]` | Initialize new `raptor.json` package |
| `raptor get <repo>[@tag]` | Clone Git dependency into `./raptor_modules/` |
| `raptor install` | Install dependencies from `raptor.json` |
| `raptor serve [--port 8080]` | Start WebAssembly in-browser playground server |
| `raptor test [t/]` | Run TAP test harness (like `prove`) |
| `raptor doc <topic>` | Read terminal markdown manual (`operators`, `tui`, `structs`, etc.) |
| `raptor pack <script.rp> -o <app.exe>` | Package script into standalone executable |
| `raptor weave <file.pod>` | Generate woven Markdown documentation |
| `raptor tangle <file.pod>` | Extract tangled source files |
| `raptor stitch <file.pod> <source>` | Reverse-tangle edited code back into POD |
| `raptorhp -S <host:port>` | Start PHP-style development HTTP server |
