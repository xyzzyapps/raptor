# Raptor Documentation Manual: WebAssembly, Canvas 2D & WebAudio

## 1. WebAssembly Compilation Overview

Raptor compiles directly into 100% standalone WebAssembly (`GOOS=js GOARCH=wasm`) with zero server-side dependencies. It runs completely client-side in modern web browsers (Chrome, Firefox, Safari, Edge):

```powershell
$env:GOOS="js"; $env:GOARCH="wasm"; go build -o web/raptor.wasm ./cmd/wasm; $env:GOOS=""; $env:GOARCH=""
```

Launch the local interactive playground:
```powershell
raptor serve --port 8080
# Opens http://localhost:8080/
```

---

## 2. HTML5 Canvas 2D Engine

Raptor provides native built-ins for fast 2D graphics, geometry, and text rendering:

```perl
# Initialize canvas (element ID, width, height)
canvas_init("wasmCanvas", 640, 380);

# Clear viewport with background color
canvas_clear("#090d16");

# Draw shapes
canvas_draw_rect(20, 20, 200, 100, "#10b981", true);          # Filled rectangle
canvas_draw_rect(20, 20, 200, 100, "#06b6d4", false);         # Stroked border
canvas_draw_circle(320, 190, 45, "#f59e0b", true);             # Filled circle
canvas_draw_line(50, 50, 400, 300, "#a855f7", 2.5);            # Radiant line

# Render text
canvas_draw_text("Raptor WebAssembly 2D", 30, 45, 18, "#ffffff");
```

---

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

---

## 4. WebAudio API Sound Synthesizer

Synthesize real-time sound effects, musical notes, and chords:

```perl
# Initialize audio context
audio_init();

# Play single tone (frequency in Hz, duration in seconds, waveform)
# Waveforms: "sine", "triangle", "square", "sawtooth"
audio_play_tone(440.0, 0.25, "sine");       # Concert A (440 Hz)
audio_play_tone(523.25, 0.35, "triangle");   # C5 (523.25 Hz)

# Play melodic sequences
my @frequencies = [261.63, 329.63, 392.00, 523.25];   # C Major Arpeggio
my @durations   = [0.15, 0.15, 0.15, 0.30];

audio_play_melody(@frequencies, @durations);
```

---

## 5. Fast JSON Interoperability

Exchange complex structured data between the WebAssembly runtime and JavaScript:

```perl
my %telemetry = {
    "engine"    => "Raptor WebAssembly",
    "timestamp" => time(),
    "nodes"     => [ { id => 1, x => 100, y => 50 }, { id => 2, x => 200, y => 80 } ],
    "active"    => true
};

# Serialize to JSON string
my $json_payload = to_json(%telemetry);
say $json_payload;

# Deserialize back into native data structure
my $parsed = from_json($json_payload);
say "Parsed engine name: ", $parsed{"engine"};
```
