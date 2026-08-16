# Raptor Documentation Manual: WebAssembly, Canvas 2D & WebAudio

## 1. WebAssembly Compilation Overview

Raptor compiles directly into 100% standalone WebAssembly (`GOOS=js GOARCH=wasm`) with zero server-side dependencies. It runs completely client-side in modern web browsers (Chrome, Firefox, Safari, Edge):

```powershell
# Prefer TinyGo when `tinygo` is on PATH; force a toolchain with --wasm-compiler
raptor --wasm --wasm-compiler=tinygo -o web/raptor.wasm
raptor --wasm --wasm-compiler=go -o web/raptor.wasm

# Also writes raptor_bridge.js + wasm_exec.js next to the .wasm
# (canvas / WebAudio / WebGL / WebGPU stubs). The WASM binary evals the
# same stubs if the page forgot to include them.
```

Launch the local interactive playground:
```powershell
raptor serve --port 8080
# Opens http://localhost:8080/
```

## 2. HTML5 Canvas 2D Engine

Raptor provides native built-ins for fast 2D graphics, geometry, and text rendering:

```perl
# Initialize canvas (element ID, width, height)
canvas_init("wasmCanvas", 640, 380);

# Clear viewport with background color
canvas_clear("#090d16");

# Draw shapes
canvas_draw_rect(20, 20, 200, 100, "#10b981", True);          # Filled rectangle
canvas_draw_rect(20, 20, 200, 100, "#06b6d4", False);         # Stroked border
canvas_draw_circle(320, 190, 45, "#f59e0b", True);             # Filled circle
canvas_draw_line(50, 50, 400, 300, "#a855f7", 2.5);            # Radiant line

# Render text
canvas_draw_text("Raptor WebAssembly 2D", 30, 45, 18, "#ffffff");
```

## 3. Dynamic DOM Manipulation

Interact with the host webpage DOM tree directly from Raptor scripts:

```perl
# Get text content of an element
my $val = dom_get("#statusBadge");

# Update element text or inner HTML
dom_set_text("#title", "Simulated Particle System");
dom_set_html("#output", "<strong>Status:</strong> All 10 nodes active");

# Dynamically construct and append new DOM elements
dom_create("#container", "div", "New telemetry record", "card-item");
```

## 4. WebAudio node graph (AudioContext timeline)

The tour builds the graph in Raptor. JS only wraps `AudioNode` / `AudioParam`. Notes are scheduled on `AudioContext.currentTime` (not `setTimeout`).

```perl
my $ctx = audio_context_create();
my $t0 = audio_get_current_time($ctx);
my $osc = audio_create_oscillator($ctx);
my $env = audio_create_gain($ctx);
my $lp  = audio_create_biquad_filter($ctx, "lowpass");
audio_set_osc_type($osc, "triangle");
audio_set_frequency($osc, 261.63, $t0);          # C4
audio_set_filter_freq($lp, 1800.0, $t0);
audio_set_gain($env, 0.0001, $t0);
audio_gain_ramp_exp($env, 0.8, $t0 + 0.02);
audio_gain_ramp_exp($env, 0.0001, $t0 + 0.18);
audio_connect($osc, $env);
audio_connect($env, $lp);
audio_connect_destination($lp, $ctx);
audio_osc_start($osc, $t0);
audio_osc_stop($osc, $t0 + 0.2);
```

Also: compressor, delay, stereo panner, analyser (`audio_get_spectrum`), buffer sources, LFO via `audio_connect_param($lfoGain, $filter, "frequency")`.

`audio_play_tone` / `audio_play_melody` compose that same graph (C4 E4 G4 B4 C5 in the tour).

## 4b. WebGPU tiny LLM

```perl
webgpu_init("wasmWebGPUCanvas", 640, 320);
my $m = llm_tiny_load();
say llm_tiny_generate($m, "raptor is ", 48, 0.7);
webgpu_draw_logits(llm_tiny_logits($m, "raptor is "));
```

On desktop the same `llm_tiny_*` builtins run on CPU. Tensor graphs can also use the GGML C API (`ggml_init`, `ggml_mul_mat`, `ggml_graph_compute_with_ctx`).

## 5. Fast JSON Interoperability

Exchange complex structured data between the WebAssembly runtime and JavaScript:

```perl
my %telemetry = {
    "engine"    => "Raptor WebAssembly",
    "timestamp" => time(),
    "nodes"     => [ { "id" => 1, "x" => 100, "y" => 50 }, { "id" => 2, "x" => 200, "y" => 80 } ],
    "active"    => True
};

# Serialize to JSON string
my $json_payload = to_json(%telemetry);
say $json_payload;

# Deserialize back into native data structure
my $parsed = from_json($json_payload);
say "Parsed engine name: " ~ $parsed{"engine"};
```
