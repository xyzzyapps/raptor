// ==============================================================================
// A Tour of Raptor - 1:1 Classic Tour of Go Interactive WebAssembly Engine
// ==============================================================================

let audioCtx = null;
window.audioCtx = null;
let canvasContexts = [];
let audioContexts = [];
let audioOscillators = [];
let audioGains = [];
let audioFilters = [];

let gl = null;
let glShaders = [];
let glPrograms = [];
let glBuffers = [];
let glUniformLocs = [];
let webglAnimId = null;
let webglRotX = 25.0;
let webglRotY = 45.0;
let webglRotZ = 15.0;
let isWebglAnimating = false;
let isWebglDragging = false;
let currentUModelViewLoc = -1;

// Browser Bridge for WebAssembly builtins (Canvas 2D, WebGL 3D, DOM, WebAudio)
window.raptorBridge = {
  // --- Low-Level HTML5 Canvas 2D Built-in Registry & Primitives ---
  canvasGetContext: function(canvasId, width, height) {
    const canvas = document.getElementById(canvasId) || document.getElementById('wasmCanvas');
    if (!canvas) {
      console.error("[raptorBridge] Canvas element not found:", canvasId);
      return 0;
    }
    if (width) canvas.width = width;
    if (height) canvas.height = height;
    const ctx = canvas.getContext('2d');
    canvasContexts = [ctx];
    switchToTab('tabCanvas', 'canvasView');
    console.log("[raptorBridge] Canvas 2D context created on", canvas.id, `${canvas.width}x${canvas.height}`);
    return 0;
  },

  canvasSetFillStyle: function(ctxId, color) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.fillStyle = color;
  },

  canvasSetStrokeStyle: function(ctxId, color) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.strokeStyle = color;
  },

  canvasSetLineWidth: function(ctxId, lw) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.lineWidth = lw;
  },

  canvasSetFont: function(ctxId, font) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.font = font;
  },

  canvasFillRect: function(ctxId, x, y, w, h) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.fillRect(x, y, w, h);
  },

  canvasStrokeRect: function(ctxId, x, y, w, h) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.strokeRect(x, y, w, h);
  },

  canvasClearRect: function(ctxId, x, y, w, h) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.clearRect(x, y, w, h);
  },

  canvasBeginPath: function(ctxId) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.beginPath();
  },

  canvasClosePath: function(ctxId) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.closePath();
  },

  canvasMoveTo: function(ctxId, x, y) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.moveTo(x, y);
  },

  canvasLineTo: function(ctxId, x, y) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.lineTo(x, y);
  },

  canvasArc: function(ctxId, x, y, r, sAngle, eAngle) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.arc(x, y, r, sAngle, eAngle);
  },

  canvasStroke: function(ctxId) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.stroke();
  },

  canvasFill: function(ctxId) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.fill();
  },

  canvasFillText: function(ctxId, text, x, y) {
    const ctx = canvasContexts[ctxId];
    if (ctx) ctx.fillText(text, x, y);
  },

  // --- Low-Level WebAudio DSP Node Built-ins ---
  initAudio: function() {
    if (!audioCtx) {
      const AudioContext = window.AudioContext || window.webkitAudioContext;
      audioCtx = new AudioContext();
    }
    if (audioCtx.state === 'suspended') {
      audioCtx.resume().catch(() => {});
    }
    console.log("[raptorBridge] initAudio() state:", audioCtx.state);
  },

  playTone: function(frequency, durationSec, waveType) {
    window.raptorBridge.initAudio();
    if (!audioCtx) return;

    const osc = audioCtx.createOscillator();
    const gain = audioCtx.createGain();

    osc.type = waveType || 'triangle';
    osc.frequency.setValueAtTime(frequency || 440, audioCtx.currentTime);

    gain.gain.setValueAtTime(0.18, audioCtx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.0001, audioCtx.currentTime + (durationSec || 0.25));

    osc.connect(gain);
    gain.connect(audioCtx.destination);

    osc.start();
    osc.stop(audioCtx.currentTime + (durationSec || 0.25));
  },

  playMelody: function(frequencies, durations) {
    window.raptorBridge.initAudio();
    if (!audioCtx) return;
    const freqs = Array.isArray(frequencies) ? frequencies : Array.from(frequencies);
    const durs = (durations && (Array.isArray(durations) || durations.length !== undefined)) ? Array.from(durations) : [];

    let timeOffset = 0;
    freqs.forEach((freq, idx) => {
      const dur = (durs && durs[idx]) ? durs[idx] : 0.18;
      setTimeout(() => {
        window.raptorBridge.playTone(freq, dur, 'triangle');
      }, timeOffset * 1000);
      timeOffset += dur;
    });
  },

  audioContextCreate: function() {
    const AudioContext = window.AudioContext || window.webkitAudioContext;
    const ctx = new AudioContext();
    if (ctx.state === 'suspended') {
      ctx.resume().catch(() => {});
    }
    audioContexts = [ctx];
    audioOscillators = [];
    audioGains = [];
    audioFilters = [];
    console.log("[raptorBridge] WebAudio AudioContext created on state:", ctx.state, "sampleRate:", ctx.sampleRate);
    return 0;
  },

  audioGetCurrentTime: function(ctxId) {
    const ctx = audioContexts[ctxId] || audioContexts[0];
    return ctx ? ctx.currentTime : 0.0;
  },

  audioCreateOscillator: function(ctxId) {
    const ctx = audioContexts[ctxId] || audioContexts[0];
    if (!ctx) return 0;
    const osc = ctx.createOscillator();
    audioOscillators.push(osc);
    return audioOscillators.length - 1;
  },

  audioCreateGain: function(ctxId) {
    const ctx = audioContexts[ctxId] || audioContexts[0];
    if (!ctx) return 0;
    const gain = ctx.createGain();
    audioGains.push(gain);
    return audioGains.length - 1;
  },

  audioCreateBiquadFilter: function(ctxId, filterType) {
    const ctx = audioContexts[ctxId] || audioContexts[0];
    if (!ctx) return 0;
    const filter = ctx.createBiquadFilter();
    filter.type = filterType || 'lowpass';
    audioFilters.push(filter);
    return audioFilters.length - 1;
  },

  audioConnect: function(srcId, dstId) {
    const src = audioOscillators[srcId] || audioGains[srcId] || audioFilters[srcId];
    const dst = audioGains[dstId] || audioFilters[dstId] || audioOscillators[dstId];
    if (src && dst) {
      src.connect(dst);
    }
  },

  audioConnectDestination: function(srcId, ctxId) {
    const src = audioOscillators[srcId] || audioGains[srcId] || audioFilters[srcId];
    const ctx = audioContexts[ctxId] || audioContexts[0];
    if (src && ctx) {
      src.connect(ctx.destination);
    }
  },

  audioSetOscType: function(oscId, waveType) {
    const osc = audioOscillators[oscId];
    if (osc) osc.type = waveType || 'sine';
  },

  audioSetFrequency: function(oscId, freq, timeOffset) {
    const osc = audioOscillators[oscId];
    if (osc) {
      const t = timeOffset || 0.0;
      osc.frequency.setValueAtTime(freq, t);
    }
  },

  audioSetGain: function(gainId, gainVal, timeOffset) {
    const gain = audioGains[gainId];
    if (gain) {
      const t = timeOffset || 0.0;
      gain.gain.setValueAtTime(gainVal, t);
    }
  },

  audioGainRampExp: function(gainId, targetVal, endTime) {
    const gain = audioGains[gainId];
    if (gain) {
      const v = Math.max(targetVal, 0.00001);
      gain.gain.exponentialRampToValueAtTime(v, endTime);
    }
  },

  audioGainRampLinear: function(gainId, targetVal, endTime) {
    const gain = audioGains[gainId];
    if (gain) {
      gain.gain.linearRampToValueAtTime(targetVal, endTime);
    }
  },

  audioSetFilterFreq: function(filterId, freq, timeOffset) {
    const filter = audioFilters[filterId];
    if (filter) {
      filter.frequency.setValueAtTime(freq, timeOffset || 0.0);
    }
  },

  audioOscStart: function(oscId, startTime) {
    const osc = audioOscillators[oscId];
    if (osc) {
      audioContexts.forEach(c => {
        if (c.state === 'suspended') c.resume().catch(() => {});
      });
      try {
        osc.start(startTime || 0.0);
      } catch (err) {
        console.warn("audioOscStart:", err);
      }
    }
  },

  audioOscStop: function(oscId, stopTime) {
    const osc = audioOscillators[oscId];
    if (osc) {
      try {
        osc.stop(stopTime || 0.0);
      } catch (err) {
        console.warn("audioOscStop:", err);
      }
    }
  },

  // --- Low-Level WebGL GPU Hardware Accelerated Bindings ---
  glInit: function(canvasId, width, height) {
    const canvas = document.getElementById(canvasId) || document.getElementById('wasmWebGLCanvas');
    if (!canvas) {
      console.error("[raptorBridge] WebGL canvas not found:", canvasId);
      return;
    }
    if (width) canvas.width = width;
    if (height) canvas.height = height;

    gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
    if (!gl) {
      console.error("[raptorBridge] WebGL context creation failed");
      return;
    }

    glShaders = [];
    glPrograms = [];
    glBuffers = [];
    glUniformLocs = [];

    gl.viewport(0, 0, canvas.width, canvas.height);
    window.raptorBridge.setupWebGLDragControls(canvas);
    switchToTab('tabWebGL', 'webglView');
    console.log("[raptorBridge] WebGL 3D initialized on", canvas.id, `${canvas.width}x${canvas.height}`);
  },

  glClearColor: function(r, g, b, a) {
    if (gl) gl.clearColor(r, g, b, a);
  },

  glClear: function() {
    if (gl) gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
  },

  glEnableDepthTest: function() {
    if (gl) gl.enable(gl.DEPTH_TEST);
  },

  glCreateShader: function(typeStr) {
    if (!gl) return 0;
    const type = (typeStr === 'FRAGMENT') ? gl.FRAGMENT_SHADER : gl.VERTEX_SHADER;
    const shader = gl.createShader(type);
    glShaders.push(shader);
    return glShaders.length - 1;
  },

  glShaderSource: function(shaderId, src) {
    if (gl && glShaders[shaderId]) {
      gl.shaderSource(glShaders[shaderId], src);
    }
  },

  glCompileShader: function(shaderId) {
    if (gl && glShaders[shaderId]) {
      gl.compileShader(glShaders[shaderId]);
      if (!gl.getShaderParameter(glShaders[shaderId], gl.COMPILE_STATUS)) {
        console.error("Shader compile error: " + gl.getShaderInfoLog(glShaders[shaderId]));
      }
    }
  },

  glCreateProgram: function() {
    if (!gl) return 0;
    const prog = gl.createProgram();
    glPrograms.push(prog);
    return glPrograms.length - 1;
  },

  glAttachShader: function(progId, shaderId) {
    if (gl && glPrograms[progId] && glShaders[shaderId]) {
      gl.attachShader(glPrograms[progId], glShaders[shaderId]);
    }
  },

  glLinkProgram: function(progId) {
    if (gl && glPrograms[progId]) {
      gl.linkProgram(glPrograms[progId]);
      if (!gl.getProgramParameter(glPrograms[progId], gl.LINK_STATUS)) {
        console.error("Program link error: " + gl.getProgramInfoLog(glPrograms[progId]));
      }
    }
  },

  glUseProgram: function(progId) {
    if (gl && glPrograms[progId]) {
      gl.useProgram(glPrograms[progId]);
    }
  },

  glGetAttribLocation: function(progId, name) {
    if (!gl || !glPrograms[progId]) return -1;
    return gl.getAttribLocation(glPrograms[progId], name);
  },

  glGetUniformLocation: function(progId, name) {
    if (!gl || !glPrograms[progId]) return -1;
    const loc = gl.getUniformLocation(glPrograms[progId], name);
    glUniformLocs.push(loc);
    if (name === 'uMVMatrix') {
      currentUModelViewLoc = glUniformLocs.length - 1;
    }
    return glUniformLocs.length - 1;
  },

  glEnableVertexAttribArray: function(loc) {
    if (gl && loc >= 0) gl.enableVertexAttribArray(loc);
  },

  glCreateBuffer: function() {
    if (!gl) return 0;
    const buf = gl.createBuffer();
    glBuffers.push(buf);
    return glBuffers.length - 1;
  },

  glBindBuffer: function(targetStr, bufId) {
    if (!gl || !glBuffers[bufId]) return;
    const target = (targetStr === 'ELEMENT') ? gl.ELEMENT_ARRAY_BUFFER : gl.ARRAY_BUFFER;
    gl.bindBuffer(target, glBuffers[bufId]);
  },

  glBufferData: function(targetStr, dataArray) {
    if (!gl || !dataArray) return;
    const arr = Array.isArray(dataArray) ? dataArray : Array.from(dataArray);
    const target = (targetStr === 'ELEMENT') ? gl.ELEMENT_ARRAY_BUFFER : gl.ARRAY_BUFFER;
    if (targetStr === 'ELEMENT') {
      gl.bufferData(target, new Uint16Array(arr), gl.STATIC_DRAW);
    } else {
      gl.bufferData(target, new Float32Array(arr), gl.STATIC_DRAW);
    }
  },

  glVertexAttribPointer: function(loc, size) {
    if (gl && loc >= 0) {
      gl.vertexAttribPointer(loc, size, gl.FLOAT, false, 0, 0);
    }
  },

  glUniformMatrix4fv: function(locId, matrixArray) {
    if (gl && glUniformLocs[locId] && matrixArray) {
      const arr = Array.isArray(matrixArray) ? matrixArray : Array.from(matrixArray);
      gl.uniformMatrix4fv(glUniformLocs[locId], false, new Float32Array(arr));
    }
  },

  glDrawElements: function(count) {
    if (gl) {
      gl.drawElements(gl.TRIANGLES, count || 36, gl.UNSIGNED_SHORT, 0);
    }
  },

  glStartAnimation: function() {
    if (webglAnimId) cancelAnimationFrame(webglAnimId);
    isWebglAnimating = true;

    function renderLoop() {
      if (isWebglAnimating && gl) {
        if (!isWebglDragging) {
          webglRotX += 0.8;
          webglRotY += 1.2;
        }

        // Calculate trigonometric rotation matrix for live frame
        const radX = webglRotX * Math.PI / 180;
        const radY = webglRotY * Math.PI / 180;
        const radZ = webglRotZ * Math.PI / 180;

        const cx = Math.cos(radX), sx = Math.sin(radX);
        const cy = Math.cos(radY), sy = Math.sin(radY);
        const cz = Math.cos(radZ), sz = Math.sin(radZ);

        const mvMatrix = new Float32Array([
          cy * cz, cx * sz + sx * sy * cz, sx * sz - cx * sy * cz, 0,
          -cy * sz, cx * cz - sx * sy * sz, sx * cz + cx * sy * sz, 0,
          sy, -sx * cy, cx * cy, 0,
          0, 0, -4.2, 1
        ]);

        if (currentUModelViewLoc >= 0 && glUniformLocs[currentUModelViewLoc]) {
          gl.uniformMatrix4fv(glUniformLocs[currentUModelViewLoc], false, mvMatrix);
        }

        gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
        gl.drawElements(gl.TRIANGLES, 36, gl.UNSIGNED_SHORT, 0);

        webglAnimId = requestAnimationFrame(renderLoop);
      }
    }
    webglAnimId = requestAnimationFrame(renderLoop);
  },

  setupWebGLDragControls: function(canvas) {
    if (!canvas || canvas.hasDragControls) return;
    canvas.hasDragControls = true;

    let lastMouseX = 0;
    let lastMouseY = 0;

    canvas.addEventListener('mousedown', (e) => {
      isWebglDragging = true;
      lastMouseX = e.clientX;
      lastMouseY = e.clientY;
    });

    window.addEventListener('mousemove', (e) => {
      if (!isWebglDragging) return;
      const deltaX = e.clientX - lastMouseX;
      const deltaY = e.clientY - lastMouseY;
      webglRotY += deltaX * 0.6;
      webglRotX += deltaY * 0.6;
      lastMouseX = e.clientX;
      lastMouseY = e.clientY;
    });

    window.addEventListener('mouseup', () => {
      isWebglDragging = false;
    });
  }
};

// ==============================================================================
// Comprehensive 13-Lesson Interactive Go Tour Content
// ==============================================================================
const tourLessons = [
  {
    title: "1. Language Basics & Operators",
    desc: `
      <p>Welcome to a tour of the <strong>Raptor</strong> programming language.</p>
      <p>Raptor is a high-performance procedural execution platform and dynamic language (Perl 5 subset of Raku without OO overhead). Variables use standard sigils (<code>$</code>, <code>@</code>, <code>%</code>).</p>
      <p>Raptor features rich built-in operators: defined-or (<code>//</code>), exponentiation (<code>**</code>), string repetition (<code>x</code>), list replication (<code>xx</code>), and chained comparisons (<code>0 &lt;= $x &lt;= 100</code>).</p>
      <p>Click <strong>Run</strong> (or press <kbd>Shift + Enter</kbd>) to compile and run in WebAssembly.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Sigils, Exponentiation & Defined-Or Defaulting
my $base = 2;
my $power = 10;
my $exponent = $base ** $power;
say "2 ** 10 = ", $exponent;

my $config = Nil;
my $timeout = $config // 5000;
say "Timeout (defined-or): ", $timeout, " ms";

# 2. String & List Replication Operators
my $divider = "=" x 35;
say $divider;

my @tags = ["alpha"] xx 3;
say "Replicated tags: ", @tags;

# 3. Chained Comparisons
my $val = 42;
if 10 <= $val <= 50 {
    say "Value $val is within range [10, 50]!";
}
`
  },

  {
    title: "2. Dynamic Subsets & Continuous Invariants",
    desc: `
      <p>Raptor replaces heavyweight OOP with <strong>Dynamic Subsets</strong> and <strong>Refinement Predicates</strong>.</p>
      <p>A subset defines a runtime type refinement constrained by a boolean block (<code>where { ... }</code>). Invariants can be attached directly to variables (<code>my $score where { $_ &gt;= 0 } = 100;</code>) or multiple-dispatch subroutines.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Refinement Types with 'subset'
subset Positive where { $_ > 0 };
subset Even where { $_ % 2 == 0 };
subset PortNumber where { 1 <= $_ <= 65535 };

# 2. Invariant-constrained typed variables
my Positive $score = 100;
my PortNumber $serverPort = 8080;
say "Verified Score: ", $score;
say "Verified Server Port: ", $serverPort;

# 3. Predicate Multiple Dispatch
multi sub handle_request(PortNumber $p where { $_ == 80 || $_ == 443 }) {
    say "Handling standard Web traffic on port: ", $p;
}

multi sub handle_request(PortNumber $p) {
    say "Handling custom internal service on port: ", $p;
}

handle_request(443);
handle_request(8080);
`
  },

  {
    title: "3. C-ABI Struct Records & Overloading",
    desc: `
      <p>Raptor features C-compatible contiguous memory <code>struct</code> records with O(1) field offsets.</p>
      <p>Structs can store first-class function pointer fields (closures) and support custom operator overloading via <code>multi sub infix:&lt;+&gt;</code>.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. C-ABI Compound Struct Record
struct Vector2 {
    num64 $x;
    num64 $y;
}

# 2. Operator Overloading on Structs
multi sub infix:<+>(Vector2 $a, Vector2 $b) {
    my $res = Vector2.new();
    $res.x = $a.x + $b.x;
    $res.y = $a.y + $b.y;
    return $res;
}

# 3. Structs with Closure / Function Pointer Fields
struct ActionButton {
    Str $label;
    Any $onClick;
}

my $v1 = Vector2.new();
$v1.x = 100.5;
$v1.y = 50.0;

my $v2 = Vector2.new();
$v2.x = 25.5;
$v2.y = 75.0;

my $sum = $v1 + $v2;
say "Vector 1 + Vector 2 = (", $sum.x, ", ", $sum.y, ")";

my $btn = ActionButton.new();
$btn.label = "Execute";
$btn.onClick = sub ($val) { say "Button [", $btn.label, "] triggered with: ", $val; };
$btn.onClick(1337);
`
  },

  {
    title: "4. Uniform Function Call Syntax (UFCS)",
    desc: `
      <p>Raptor embraces <strong>Uniform Function Call Syntax (UFCS)</strong> across the entire language.</p>
      <p>Any subroutine <code>foo($target, @args)</code> can be invoked seamlessly as <code>$target.foo(@args)</code>, enabling fluid functional pipelines without class hierarchies.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Built-in Functional Pipelines via UFCS
my @numbers = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

my @evens = @numbers.grep(sub ($x) { return $x % 2 == 0; });
my @squared = @evens.map(sub ($x) { return $x * $x; });
my @sortedDesc = @squared.sort().reverse();

say "Original:  ", @numbers;
say "Evens:     ", @evens;
say "Squared:   ", @squared;
say "Reversed:  ", @sortedDesc;

# 2. Custom Subroutines callable via UFCS
sub add_prefix($str, $prefix) {
    return $prefix ~ " :: " ~ $str;
}

my $msg = "System Ready".add_prefix("[RAPTOR]");
say $msg;
`
  },

  {
    title: "5. Autothreading Quantum Junctions",
    desc: `
      <p>Junctions combine multiple values into a single superposition state: <code>any</code>, <code>all</code>, <code>one</code>, or <code>none</code>.</p>
      <p>When evaluated in boolean conditionals or <code>given / when</code> pattern matching, the condition checks across all quantum states concurrently.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Quantum Junction Conditionals
my $target = 25;

if $target == any(10, 20, 25, 30) {
    say "Target matched inside any(10, 20, 25, 30)";
}

my @scores = [85, 92, 78, 95];
if all(@scores) > 70 {
    say "All test scores exceeded 70!";
}

# 2. Smartmatching & Given / When
given $target {
    when any(1..10)   { say "Small value"; }
    when any(20..30)  { say "Target matched 20..30 range!"; }
    default           { say "Out of range"; }
}
`
  },

  {
    title: "6. Signature Destructuring & Fast JSON",
    desc: `
      <p>Raptor allows deep parameter destructuring of lists (head & tail) and associative hashes directly in subroutine signatures.</p>
      <p>JSON serialization and parsing are built into the core language via <code>to_json</code> and <code>from_json</code>.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Array Parameter Destructuring (Head & Tail)
sub inspect_list([$head, *@tail]) {
    say "Head element: ", $head;
    say "Tail elements: ", @tail;
}

inspect_list([100, 200, 300, 400]);

# 2. High-Performance JSON Interop
my %payload = {
    "engine"  => "Raptor",
    "version" => "1.0.0",
    "threads" => 8,
    "active"  => True
};

my $jsonStr = to_json(%payload);
say "Encoded JSON:\n", $jsonStr;

my %decoded = from_json($jsonStr);
say "Decoded engine: ", %decoded{"engine"};
`
  },

  {
    title: "7. Gather / Take Generators & Lazy Lists",
    desc: `
      <p>Raptor features first-class coroutine generators using <code>gather { ... take ... }</code>.</p>
      <p>Generators yield values dynamically, allowing elegant creation of mathematical sequences and filtered data streams.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Gather / Take Generator
my @cubes = gather {
    for 1..6 -> $n {
        take $n * $n * $n;
    }
};

say "Generated Cubes: ", @cubes;

# 2. Filtering Stream Generator
my @filtered = gather {
    for 1..20 -> $x {
        if $x % 3 == 0 || $x % 5 == 0 {
            take $x;
        }
    }
};

say "Multiples of 3 or 5: ", @filtered;
`
  },

  {
    title: "8. Design-by-Contract & Verification",
    desc: `
      <p>Raptor incorporates formal <strong>Design-by-Contract</strong> specifications and automated <strong>Property-Based Verification</strong> (QuickCheck-style randomized test generation).</p>
      <p>Click <strong>Run</strong> to verify contracts and execute 100 randomized property trials.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Contract-Enforced Refinement Subroutine
subset NonZero where { $_ != 0 };

sub safe_divide($a, NonZero $b) {
    return $a / $b;
}

say "safe_divide(42, 6) = ", safe_divide(42, 6);

# 2. QuickCheck-Style Randomized Property Verification
property "Addition Commutativity", sub ($a, $b) {
    return ($a + $b) == ($b + $a);
};
`
  },

  {
    title: "9. PodLit Literate Programming",
    desc: `
      <p>Raptor includes the <strong>PodLit</strong> literate programming engine supporting Donald Knuth-style chunk tangling (<code>pod_tangle</code>), documentation weaving (<code>pod_weave</code>), and bidirectional code stitching (<code>pod_stitch</code>).</p>
      <p>Click <strong>Run</strong> to weave the specification to Markdown and tangle the executable source files into stdout.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Define Literate Pod Document with Chunk and Target File
my $pod = '=pod

=head1 Vector Mathematics Subsystem

This document specifies the 2D vector calculation routine.

=chunk <vector-ops> :file "lib/Vector.rp"
sub vector_dot($x1, $y1, $x2, $y2) {
    return ($x1 * $x2) + ($y1 * $y2);
}
=end chunk

=cut
';

# 2. Weave Documentation into Markdown Format
my $markdown = pod_weave($pod);
say "=== 1. Woven Markdown Documentation ===";
say $markdown;

# 3. Tangle Executable Source Code Files
my %tangledFiles = pod_tangle($pod);
say "=== 2. Tangled Source Code Files ===";
say %tangledFiles;

say "=== Literate Programming Pipeline Verified ===";
`
  },

  {
    title: "10. HTML5 Canvas 2D Graphics Engine",
    desc: `
      <p>Raptor communicates directly with HTML5 2D Canvas contexts via procedural drawing primitives.</p>
      <p>The entire radar HUD, coordinate grid, and geometric math are evaluated in <strong>pure Raptor code</strong>.</p>
    `,
    defaultTab: "tabCanvas",
    defaultView: "canvasView",
    code: `# ==============================================================================
# Pure Raptor HTML5 Canvas 2D Procedural Graphics Engine (640x320)
# ==============================================================================

# 1. Acquire 2D Canvas Context (640x320)
my $ctx = canvas_get_context("wasmCanvas", 640, 320);

# 2. Clear Canvas Background (Clean White Palette)
canvas_set_fill_style($ctx, "#ffffff");
canvas_fill_rect($ctx, 0, 0, 640, 320);

# 3. Draw Background Coordinate Grid in Raptor
canvas_set_stroke_style($ctx, "#f1f5f9");
canvas_set_line_width($ctx, 1.0);

for 0..12 -> $i {
    my $x = $i * 50;
    canvas_begin_path($ctx);
    canvas_move_to($ctx, $x, 0);
    canvas_line_to($ctx, $x, 320);
    canvas_stroke($ctx);
}

# 4. Draw HUD Dashboard Panels in Raptor
canvas_set_fill_style($ctx, "#f8fafc");
canvas_fill_rect($ctx, 50, 35, 540, 250);

canvas_set_stroke_style($ctx, "#007d9c");
canvas_set_line_width($ctx, 1.5);
canvas_stroke_rect($ctx, 50, 35, 540, 250);

# 5. Draw Trigonometric Radar Concentric Rings & Crosshairs in Raptor
my $pi = 3.141592653589793;
my $cx = 320.0;
my $cy = 160.0;

canvas_set_stroke_style($ctx, "#00add8");
canvas_set_line_width($ctx, 1.5);

# Outer Ring (r=70)
canvas_begin_path($ctx);
canvas_arc($ctx, $cx, $cy, 70.0, 0.0, 2.0 * $pi);
canvas_stroke($ctx);

# Middle Ring (r=40)
canvas_begin_path($ctx);
canvas_arc($ctx, $cx, $cy, 40.0, 0.0, 2.0 * $pi);
canvas_stroke($ctx);

# Center Target (r=8)
canvas_set_fill_style($ctx, "#059669");
canvas_begin_path($ctx);
canvas_arc($ctx, $cx, $cy, 8.0, 0.0, 2.0 * $pi);
canvas_fill($ctx);

# Procedural Radar Blips via Trigonometry (angle = 35 deg)
my $a1 = 35.0 * $pi / 180.0;
my $bx1 = $cx + 50.0 * cos($a1);
my $by1 = $cy - 50.0 * sin($a1);
canvas_set_fill_style($ctx, "#0284c7");
canvas_begin_path($ctx);
canvas_arc($ctx, $bx1, $by1, 5.0, 0.0, 2.0 * $pi);
canvas_fill($ctx);

# 6. Typography & HUD Status Labels rendered in Raptor
canvas_set_font($ctx, "bold 15px 'Fira Code', monospace");
canvas_set_fill_style($ctx, "#007d9c");
canvas_fill_text($ctx, "RAPTOR PROCEDURAL 2D CANVAS", 185, 75);

canvas_set_font($ctx, "11px 'Fira Code', monospace");
canvas_set_fill_style($ctx, "#64748b");
canvas_fill_text($ctx, "Vector Math & Primitives Evaluated in Pure Raptor", 150, 95);

canvas_set_fill_style($ctx, "#059669");
canvas_fill_text($ctx, "STATUS: ONLINE | RADAR: ACTIVE | INVARIANTS: OK", 155, 260);

say "Canvas 2D HUD generated with pure Raptor vector primitives and trigonometry!";
`
  },

  {
    title: "11. WebGL 3D Hardware Graphics",
    desc: `
      <p>Raptor compiles GLSL shaders, defines 3D geometry buffers, and computes 4x4 trigonometric transformation matrices directly in Raptor source code.</p>
      <p>Click <strong>Run</strong> to compile shaders, upload the 3D cube to GPU memory, and start continuous hardware-accelerated 3D rotation!</p>
    `,
    defaultTab: "tabWebGL",
    defaultView: "webglView",
    code: `# ==============================================================================
# Pure Raptor WebGL 3D Hardware Accelerated Geometry Engine (640x320)
# ==============================================================================

# 1. Initialize WebGL on Canvas (640x320) with Light Background & Depth Test
gl_init("wasmWebGLCanvas", 640, 320);
gl_enable_depth_test();
gl_clear_color(0.97, 0.98, 0.99, 1.0);

# 2. Compile GLSL Vertex & Fragment Shaders directly in Raptor (raw single-quoted literals)
my $vsSource = '
  attribute vec3 aPos;
  attribute vec4 aColor;
  uniform mat4 uMVMatrix;
  uniform mat4 uPMatrix;
  varying lowp vec4 vColor;
  void main(void) {
    gl_Position = uPMatrix * uMVMatrix * vec4(aPos, 1.0);
    vColor = aColor;
  }
';

my $fsSource = '
  varying lowp vec4 vColor;
  void main(void) {
    gl_FragColor = vColor;
  }
';

my $vs = gl_create_shader("VERTEX");
gl_shader_source($vs, $vsSource);
gl_compile_shader($vs);

my $fs = gl_create_shader("FRAGMENT");
gl_shader_source($fs, $fsSource);
gl_compile_shader($fs);

my $prog = gl_create_program();
gl_attach_shader($prog, $vs);
gl_attach_shader($prog, $fs);
gl_link_program($prog);
gl_use_program($prog);

# 3. Define 3D Cube Geometry, Shaded Colors & Element Indices in Raptor
my @vertices = [
  # Front
  -1.0, -1.0,  1.0,   1.0, -1.0,  1.0,   1.0,  1.0,  1.0,  -1.0,  1.0,  1.0,
  # Back
  -1.0, -1.0, -1.0,  -1.0,  1.0, -1.0,   1.0,  1.0, -1.0,   1.0, -1.0, -1.0,
  # Top
  -1.0,  1.0, -1.0,  -1.0,  1.0,  1.0,   1.0,  1.0,  1.0,   1.0,  1.0, -1.0,
  # Bottom
  -1.0, -1.0, -1.0,   1.0, -1.0, -1.0,   1.0, -1.0,  1.0,  -1.0, -1.0,  1.0,
  # Right
   1.0, -1.0, -1.0,   1.0,  1.0, -1.0,   1.0,  1.0,  1.0,   1.0, -1.0,  1.0,
  # Left
  -1.0, -1.0, -1.0,  -1.0, -1.0,  1.0,  -1.0,  1.0,  1.0,  -1.0,  1.0, -1.0
];

my @colors = [
  # 6 Vibrant Shaded Faces
  0.0, 0.49, 0.61, 1.0,  0.0, 0.49, 0.61, 1.0,  0.0, 0.49, 0.61, 1.0,  0.0, 0.49, 0.61, 1.0, # Teal Front
  0.96, 0.62, 0.04, 1.0, 0.96, 0.62, 0.04, 1.0, 0.96, 0.62, 0.04, 1.0, 0.96, 0.62, 0.04, 1.0, # Amber Back
  0.20, 0.83, 0.60, 1.0, 0.20, 0.83, 0.60, 1.0, 0.20, 0.83, 0.60, 1.0, 0.20, 0.83, 0.60, 1.0, # Emerald Top
  0.86, 0.15, 0.15, 1.0, 0.86, 0.15, 0.15, 1.0, 0.86, 0.15, 0.15, 1.0, 0.86, 0.15, 0.15, 1.0, # Red Bottom
  0.55, 0.36, 0.96, 1.0, 0.55, 0.36, 0.96, 1.0, 0.55, 0.36, 0.96, 1.0, 0.55, 0.36, 0.96, 1.0, # Purple Right
  0.00, 0.68, 0.85, 1.0, 0.00, 0.68, 0.85, 1.0, 0.00, 0.68, 0.85, 1.0, 0.00, 0.68, 0.85, 1.0  # Cyan Left
];

my @indices = [
  0, 1, 2,  0, 2, 3,    4, 5, 6,  4, 6, 7,
  8, 9,10,  8,10,11,   12,13,14, 12,14,15,
 16,17,18, 16,18,19,   20,21,22, 20,22,23
];

# 4. Upload Geometry & Color Buffers to GPU
my $posBuf = gl_create_buffer();
gl_bind_buffer("ARRAY", $posBuf);
gl_buffer_data("ARRAY", @vertices);
my $aPos = gl_get_attrib_location($prog, "aPos");
gl_enable_vertex_attrib_array($aPos);
gl_vertex_attrib_pointer($aPos, 3);

my $colBuf = gl_create_buffer();
gl_bind_buffer("ARRAY", $colBuf);
gl_buffer_data("ARRAY", @colors);
my $aColor = gl_get_attrib_location($prog, "aColor");
gl_enable_vertex_attrib_array($aColor);
gl_vertex_attrib_pointer($aColor, 4);

my $idxBuf = gl_create_buffer();
gl_bind_buffer("ELEMENT", $idxBuf);
gl_buffer_data("ELEMENT", @indices);

# 5. 3D Perspective Projection & Euler Rotation Matrix Math in Raptor
my $pi = 3.141592653589793;

sub calc_perspective_matrix($fovDeg, $aspect, $near, $far) {
    my $pi = 3.141592653589793;
    my $fovRad = $fovDeg * $pi / 180.0;
    my $f = 1.0 / tan($fovRad / 2.0);
    my $nf = 1.0 / ($near - $far);
    return [
        $f / $aspect, 0.0, 0.0, 0.0,
        0.0, $f, 0.0, 0.0,
        0.0, 0.0, ($far + $near) * $nf, -1.0,
        0.0, 0.0, (2.0 * $far * $near) * $nf, 0.0
    ];
}

sub calc_rotation_matrix($rotXDeg, $rotYDeg) {
    my $pi = 3.141592653589793;
    my $rx = $rotXDeg * $pi / 180.0;
    my $ry = $rotYDeg * $pi / 180.0;
    my $cx = cos($rx); my $sx = sin($rx);
    my $cy = cos($ry); my $sy = sin($ry);
    return [
        $cy, $sx * $sy, -$cx * $sy, 0.0,
        0.0, $cx, $sx, 0.0,
        $sy, -$sx * $cy, $cx * $cy, 0.0,
        0.0, 0.0, -4.2, 1.0
    ];
}

my $uPMatrix = gl_get_uniform_location($prog, "uPMatrix");
my $uMVMatrix = gl_get_uniform_location($prog, "uMVMatrix");

# 6. Render Initial 3D Frame & Start Continuous Hardware Rotation
my @pMat = calc_perspective_matrix(45.0, 640.0 / 320.0, 0.1, 100.0);
my @mvMat = calc_rotation_matrix(25.0, 45.0);

gl_uniform_matrix4fv($uPMatrix, @pMat);
gl_uniform_matrix4fv($uMVMatrix, @mvMat);

gl_clear();
gl_draw_elements(36);
gl_animate();

say "WebGL 3D Engine Initialized:";
say " - Shaders compiled from Raptor source strings";
say " - 3D Cube geometry & buffers uploaded to GPU";
say " - 4x4 Perspective & Trigonometric Euler rotation calculated in Raptor";
say " - Continuous 60fps hardware rotation loop active!";
`
  },

  {
    title: "12. WebAudio Waveform Synthesizer",
    desc: `
      <p>Generate audio waveforms, musical arpeggios, and synthesizers in real-time through the browser's WebAudio API directly from pure Raptor.</p>
      <p>Click <strong>Run</strong> to listen to the generated musical sequence.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# ==============================================================================
# Pure Raptor WebAudio Sound Synthesizer & Arpeggio
# ==============================================================================

# 1. Initialize WebAudio Context
audio_init();

say "--- Synthesizing WebAudio Major 7th Arpeggio ---";

# Frequencies in Hz: C4, E4, G4, B4, C5 (Major 7th Arpeggio)
my @melody = [261.63, 329.63, 392.00, 493.88, 523.25];
my @durations = [0.15, 0.15, 0.15, 0.15, 0.35];

# 2. Play musical sequence in pure Raptor
audio_play_melody(@melody, @durations);

say "WebAudio Arpeggio playing via browser WebAudio API!";
`
  },

  {
    title: "13. Full-Stack In-Browser App",
    desc: `
      <p>Combine Canvas 2D, WebGL 3D, DOM mutation, and WebAudio sound effects in a single reactive in-browser application.</p>
    `,
    defaultTab: "tabCanvas",
    defaultView: "canvasView",
    code: `# 1. Live DOM Card Construction (raw single-quoted literal)
my $html = '
  <div style="background:#ffffff; padding:1.5rem; border-radius:8px; border:1px solid #cbd5e1; box-shadow:0 4px 12px rgba(0,0,0,0.05); color:#111827;">
    <h2 style="color:#007d9c; margin:0 0 0.5rem 0;">Raptor Full-Stack Reactive App</h2>
    <p style="color:#64748b; margin:0 0 1rem 0;">DOM + Canvas + WebGL + Audio Synchronized in WebAssembly</p>
    <div style="display:flex; gap:0.5rem;">
      <span style="background:#007d9c; color:#ffffff; padding:4px 12px; border-radius:4px; font-weight:bold; font-size:0.8rem;">Invariants: OK</span>
      <span style="background:#059669; color:#ffffff; padding:4px 12px; border-radius:4px; font-weight:bold; font-size:0.8rem;">GPU: Accelerated</span>
    </div>
  </div>
';
dom_set_html("#wasmDomContainer", $html);

# 2. Render Canvas 2D Radar in pure Raptor (640x320)
my $pi = 3.141592653589793;
my $c2d = canvas_get_context("wasmCanvas", 640, 320);
canvas_set_fill_style($c2d, "#ffffff");
canvas_fill_rect($c2d, 0, 0, 640, 320);

canvas_set_stroke_style($c2d, "#059669");
canvas_set_line_width($c2d, 2.0);
canvas_begin_path($c2d);
canvas_arc($c2d, 320.0, 160.0, 60.0, 0.0, 2.0 * $pi);
canvas_stroke($c2d);

canvas_set_font($c2d, "bold 14px 'Fira Code', monospace");
canvas_set_fill_style($c2d, "#007d9c");
canvas_fill_text($c2d, "SYSTEM READY", 270, 165);

# 3. Trigger Confirmation WebAudio Synthesis Arpeggio (same melody)
audio_init();
my @melody = [261.63, 329.63, 392.00, 493.88, 523.25];
my @durations = [0.15, 0.15, 0.15, 0.15, 0.35];
audio_play_melody(@melody, @durations);

say "Full-stack application initialized: DOM, Canvas, and WebAudio active!";
`
  }
];

// Current State
let currentLessonIdx = 0;
let isWasmReady = false;

// DOM Elements
const statusDot = document.getElementById('statusDot');
const statusText = document.getElementById('statusText');
const lessonTitle = document.getElementById('lessonTitle');
const lessonDesc = document.getElementById('lessonDesc');
const lessonSelect = document.getElementById('lessonSelect');
const codeEditor = document.getElementById('codeEditor');
const lineNumbers = document.getElementById('lineNumbers');
const consoleTerminal = document.getElementById('consoleTerminal');
const replInput = document.getElementById('replInput');
const btnRun = document.getElementById('btnRun');
const btnResetCode = document.getElementById('btnResetCode');
const btnFormatCode = document.getElementById('btnFormatCode');
const btnClearConsole = document.getElementById('btnClearConsole');
const btnPrevLesson = document.getElementById('btnPrevLesson');
const btnNextLesson = document.getElementById('btnNextLesson');
const pageIndicator = document.getElementById('pageIndicator');

// Initialize Tour
function initTour() {
  loadLesson(0);
  setupEventListeners();
  setupPaneResizers();
  setupCanvasDimensionControls();
  initWasm();
}

function loadLesson(idx) {
  if (idx < 0 || idx >= tourLessons.length) return;
  currentLessonIdx = idx;
  const lesson = tourLessons[idx];

  lessonTitle.innerHTML = lesson.title;
  lessonDesc.innerHTML = lesson.desc;
  codeEditor.value = lesson.code.trim();
  updateLineNumbers();

  lessonSelect.value = idx.toString();
  pageIndicator.textContent = `${idx + 1} / ${tourLessons.length}`;

  btnPrevLesson.disabled = (idx === 0);
  btnNextLesson.disabled = (idx === tourLessons.length - 1);

  if (lesson.defaultTab && lesson.defaultView) {
    switchToTab(lesson.defaultTab, lesson.defaultView);
  }
}

function updateLineNumbers() {
  const lines = codeEditor.value.split('\n').length;
  let nums = '';
  for (let i = 1; i <= Math.max(lines, 20); i++) {
    nums += i + '<br>';
  }
  lineNumbers.innerHTML = nums;
}

function switchToTab(tabId, viewId) {
  document.querySelectorAll('.view-switch-btn').forEach(btn => btn.classList.remove('active'));
  document.querySelectorAll('.output-view-pane').forEach(p => p.classList.remove('active'));

  const tabBtn = document.getElementById(tabId);
  const viewPane = document.getElementById(viewId);
  if (tabBtn) tabBtn.classList.add('active');
  if (viewPane) viewPane.classList.add('active');
}

function appendToConsole(text, type = 'output') {
  const entry = document.createElement('div');
  entry.className = 'terminal-entry';

  if (type === 'input') {
    entry.className += ' terminal-input-line';
    entry.textContent = '> ' + text;
  } else if (type === 'error') {
    entry.className += ' terminal-error-line';
    entry.textContent = text;
  } else if (type === 'result') {
    entry.className += ' terminal-result-line';
    entry.textContent = text;
  } else {
    entry.className += ' terminal-output-line';
    entry.textContent = text;
  }

  consoleTerminal.appendChild(entry);
  consoleTerminal.scrollTop = consoleTerminal.scrollHeight;
}

// Execute Code in WebAssembly
function executeCodeString(code) {
  const evalFn = window.raptorEval || window.evalRaptor;
  if (typeof evalFn !== 'function') {
    throw new Error("WebAssembly execution function not ready");
  }

  const raw = evalFn(code);
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw);
    } catch (e) {
      return { output: raw, stdout: raw };
    }
  }
  return raw || {};
}

// Run Code in WebAssembly
function runCode() {
  const code = codeEditor.value;
  if (!code.trim()) return;

  const evalFn = window.raptorEval || window.evalRaptor;
  if (!isWasmReady || typeof evalFn !== 'function') {
    appendToConsole("Error: WebAssembly runtime is still initializing...", "error");
    return;
  }

  const lesson = tourLessons[currentLessonIdx];
  if (lesson && lesson.defaultTab) {
    switchToTab(lesson.defaultTab, lesson.defaultView);
  } else {
    switchToTab('tabConsole', 'consoleView');
  }

  try {
    const res = executeCodeString(code) || {};

    if (res.stdout || res.output) {
      appendToConsole(res.stdout || res.output, 'output');
    }
    if (res.error) {
      appendToConsole("Runtime error: " + res.error, 'error');
    } else if (res.result && res.result !== "Nil") {
      appendToConsole("=> " + res.result, 'result');
    }
  } catch (err) {
    appendToConsole("Execution exception: " + err.message, 'error');
  }
}

// Setup Event Listeners
function setupEventListeners() {
  if (btnRun) btnRun.addEventListener('click', runCode);
  if (btnResetCode) {
    btnResetCode.addEventListener('click', () => {
      const lesson = tourLessons[currentLessonIdx];
      if (lesson && lesson.code) {
        codeEditor.value = lesson.code.trim();
        updateLineNumbers();
      }
    });
  }

  if (btnClearConsole) {
    btnClearConsole.addEventListener('click', () => {
      consoleTerminal.innerHTML = '';
    });
  }

  if (btnPrevLesson) btnPrevLesson.addEventListener('click', () => loadLesson(currentLessonIdx - 1));
  if (btnNextLesson) btnNextLesson.addEventListener('click', () => loadLesson(currentLessonIdx + 1));
  if (lessonSelect) lessonSelect.addEventListener('change', (e) => loadLesson(parseInt(e.target.value, 10)));

  if (codeEditor) {
    codeEditor.addEventListener('input', updateLineNumbers);
    codeEditor.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && e.shiftKey) {
        e.preventDefault();
        runCode();
      }
      if (e.key === 'Tab') {
        e.preventDefault();
        const start = codeEditor.selectionStart;
        const end = codeEditor.selectionEnd;
        codeEditor.value = codeEditor.value.substring(0, start) + "    " + codeEditor.value.substring(end);
        codeEditor.selectionStart = codeEditor.selectionEnd = start + 4;
      }
    });
  }

  if (replInput) {
    replInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const code = replInput.value.trim();
        if (!code) return;
        replInput.value = '';
        appendToConsole(code, 'input');
        try {
          const res = executeCodeString(code);
          if (res.stdout || res.output) appendToConsole(res.stdout || res.output, 'output');
          if (res.error) appendToConsole(res.error, 'error');
          else if (res.result) appendToConsole(res.result, 'result');
        } catch (err) {
          appendToConsole(err.message, 'error');
        }
      }
    });
  }

  // Tab Switching
  const tc = document.getElementById('tabConsole');
  const tcv = document.getElementById('tabCanvas');
  const tw = document.getElementById('tabWebGL');
  const td = document.getElementById('tabDom');
  if (tc) tc.addEventListener('click', () => switchToTab('tabConsole', 'consoleView'));
  if (tcv) tcv.addEventListener('click', () => switchToTab('tabCanvas', 'canvasView'));
  if (tw) tw.addEventListener('click', () => switchToTab('tabWebGL', 'webglView'));
  if (td) td.addEventListener('click', () => switchToTab('tabDom', 'domView'));
}

// Setup Interactive Split Pane Resizers
function setupPaneResizers() {
  const container = document.getElementById('tourMainContainer');
  const narrativePane = document.getElementById('narrativePane');
  const interactivePane = document.getElementById('interactivePane');
  const resizerH = document.getElementById('resizerH');

  const editorArea = document.getElementById('editorArea');
  const outputArea = document.getElementById('outputArea');
  const resizerV = document.getElementById('resizerV');

  // Horizontal Resize (Left Narrative vs Right Interactive)
  if (resizerH && narrativePane && interactivePane) {
    let isResizingH = false;

    resizerH.addEventListener('mousedown', (e) => {
      isResizingH = true;
      resizerH.classList.add('resizing');
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
    });

    window.addEventListener('mousemove', (e) => {
      if (!isResizingH) return;
      const containerRect = container.getBoundingClientRect();
      const offset = e.clientX - containerRect.left;
      const minW = 280;
      const maxW = containerRect.width - 320;
      if (offset >= minW && offset <= maxW) {
        const leftPercent = (offset / containerRect.width) * 100;
        narrativePane.style.flex = `0 0 ${leftPercent}%`;
        interactivePane.style.flex = `0 0 ${100 - leftPercent}%`;
      }
    });

    window.addEventListener('mouseup', () => {
      if (isResizingH) {
        isResizingH = false;
        resizerH.classList.remove('resizing');
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      }
    });
  }

  // Vertical Resize (Editor Area vs Output Area)
  if (resizerV && editorArea && outputArea) {
    let isResizingV = false;

    resizerV.addEventListener('mousedown', (e) => {
      isResizingV = true;
      resizerV.classList.add('resizing');
      document.body.style.cursor = 'row-resize';
      document.body.style.userSelect = 'none';
    });

    window.addEventListener('mousemove', (e) => {
      if (!isResizingV) return;
      const interactiveRect = interactivePane.getBoundingClientRect();
      const offset = e.clientY - interactiveRect.top;
      const minH = 120;
      const maxH = interactiveRect.height - 140;
      if (offset >= minH && offset <= maxH) {
        const topPercent = (offset / interactiveRect.height) * 100;
        editorArea.style.flex = `0 0 ${topPercent}%`;
        outputArea.style.flex = `0 0 ${100 - topPercent}%`;
      }
    });

    window.addEventListener('mouseup', () => {
      if (isResizingV) {
        isResizingV = false;
        resizerV.classList.remove('resizing');
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      }
    });
  }
}

// Setup Canvas Dimension Controls
function setupCanvasDimensionControls() {
  document.querySelectorAll('.canvas-size-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      const isWebgl = btn.getAttribute('data-target') === 'webgl';
      const w = parseInt(btn.getAttribute('data-w'), 10);
      const h = parseInt(btn.getAttribute('data-h'), 10);

      const parentGroup = btn.parentElement;
      parentGroup.querySelectorAll('.canvas-size-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');

      if (isWebgl) {
        const glCanvas = document.getElementById('wasmWebGLCanvas');
        if (glCanvas) {
          glCanvas.width = w;
          glCanvas.height = h;
          if (gl) gl.viewport(0, 0, w, h);
          const lbl = document.getElementById('webglDimLabel');
          if (lbl) lbl.textContent = `${w} × ${h} px`;
        }
      } else {
        const canvas = document.getElementById('wasmCanvas');
        if (canvas) {
          canvas.width = w;
          canvas.height = h;
          const lbl = document.getElementById('canvasDimLabel');
          if (lbl) lbl.textContent = `${w} × ${h} px`;
        }
      }
    });
  });
}

// Initialize WebAssembly Runtime
function initWasm() {
  if (!WebAssembly.instantiateStreaming) {
    WebAssembly.instantiateStreaming = async (resp, importObject) => {
      const source = await (await resp).arrayBuffer();
      return await WebAssembly.instantiate(source, importObject);
    };
  }

  const go = new Go();
  WebAssembly.instantiateStreaming(fetch('raptor.wasm'), go.importObject)
    .then((result) => {
      go.run(result.instance);

      // Check until Go runtime has fully registered exports
      let retries = 0;
      const checkReady = setInterval(() => {
        retries++;
        if (typeof window.raptorEval === 'function' || typeof window.evalRaptor === 'function') {
          clearInterval(checkReady);
          isWasmReady = true;
          statusDot.className = 'status-dot ready';
          statusText.textContent = 'WebAssembly Ready';
          appendToConsole("Raptor WebAssembly Environment Initialized Successfully.", "output");
        } else if (retries > 100) {
          clearInterval(checkReady);
          statusDot.className = 'status-dot error';
          statusText.textContent = 'Init Timeout';
          appendToConsole("Timeout waiting for WebAssembly exports.", "error");
        }
      }, 30);
    })
    .catch((err) => {
      statusDot.className = 'status-dot error';
      statusText.textContent = 'Wasm Load Error';
      appendToConsole("Failed loading raptor.wasm: " + err.message, "error");
    });
}

// Start Tour when DOM is ready
document.addEventListener('DOMContentLoaded', initTour);
