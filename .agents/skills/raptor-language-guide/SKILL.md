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

## 2. Syntax & Data Structures

### 2.1 Variables & Sigils

```perl
my $scalar = "Dynamic String";
my @array  = [10, 20, 30, 40];
my %hash   = { "name" => "Ada", "role" => "Admin" };
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
    int32 $id;
    ptr $onClick;
}

my $btn = Button.new();
$btn.id = 42;
$btn.onClick = sub ($val) { say "Button " ~ $btn.id ~ " clicked with: " ~ $val; };
$btn.onClick("OK"); # Invokes closure via method syntax
```

### 2.3 Subsets & Predicate Multiple Dispatch

```perl
subset Positive where { $_ > 0 };
subset Even where { $_ % 2 == 0 };

multi sub classify(Even $n) { say "$n is Even"; }
multi sub classify(Positive $n) { say "$n is Positive"; }

# Predicate Pattern Matching in Function Signatures
multi sub fib($n where { $n <= 1 }) { return $n; }
multi sub fib($n) { return fib($n - 1) + fib($n - 2); }
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

## 3. Foreign Function Interface (NativeCall) & Raylib Graphics

### 3.1 C FFI Calling Conventions

```perl
# NativeCall bindings
sub InitWindow(int32 $w, int32 $h, string $title) returns void is native('libraylib.dll') { * }
sub WindowShouldClose() returns bool is native('libraylib.dll') { * }
sub CloseWindow() returns void is native('libraylib.dll') { * }
sub BeginDrawing() returns void is native('libraylib.dll') { * }
sub EndDrawing() returns void is native('libraylib.dll') { * }
```

## 4. WebAssembly (Wasm) Browser Runtime

Raptor compiles to pure WebAssembly (`web/raptor.wasm`) running 100% client-side in browsers:
```powershell
raptor serve --port 8080
# Opens in-browser REPL and playground at http://localhost:8080/

# WASM build: TinyGo if available, else Go. Stubs (raptor_bridge.js, wasm_exec.js) written beside the .wasm.
raptor --wasm --wasm-compiler=go -o web/raptor.wasm
raptor --wasm --wasm-compiler=tinygo -o web/raptor.wasm

# PHP-style RaptorHP server
raptor -S localhost:8000 -t .
```

### Web Built-ins:
- **Canvas 2D**: `canvas_init`, `canvas_clear`, `canvas_draw_rect`, `canvas_draw_circle`, `canvas_draw_line`, `canvas_draw_text`.
- **DOM Engine**: `dom_get`, `dom_set_text`, `dom_set_html`, `dom_create`.
- **WebAudio API**: `audio_context_create`, `audio_create_oscillator`, `audio_create_gain`, `audio_create_biquad_filter`, `audio_create_compressor`, `audio_create_delay`, `audio_connect`, `audio_connect_param`, `audio_gain_ramp_exp`, `audio_osc_start` — schedule on `audio_get_current_time`. `audio_play_melody` composes that graph (C4–C5 major 7th in the tour).
- **WebGPU tiny LLM**: `webgpu_init`, `llm_tiny_load`, `llm_tiny_generate`, `llm_tiny_logits`, `webgpu_draw_logits`.
- **GGML C API**: `ggml_init`, `ggml_new_tensor_2d`, `ggml_mul_mat` (`A^T * B`), `ggml_graph_compute_with_ctx` (software; optional `ggml.dll`).

## 5. Verification: Contracts & QuickCheck Fuzzing

```perl
sub safe_divide($a, $b) {
    PRE { $b != 0 }
    POST { $_ >= 0 }
    return $a div $b;
}

PROPERTY "addition commutativity" ($a, $b) {
    return ($a + $b) == ($b + $a);
}

plan 2;
ok 1 + 1 == 2, "sum";
ok { $b > 0 }, "block cond";
done_testing;
```

Lowercase `post` is a normal identifier (`sub post`, HTTP helpers). `PRE`/`POST`/`INVARIANT`/`CHECK`/`ASSERT` are statement or block forms; TAP (`plan`/`ok`/`is`/`isnt`/`like`/`done_testing`) uses the same no-paren / `{ }` style. Parenthesized calls still work.

## AI credit

The first prototype was written by **Gemini**. The language was mostly written by Anti Gravity — Gemini 3.6. Library bindings and PodLit were written by Gemini 3.7. Later work was modified by **Grok**.
