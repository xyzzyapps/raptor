// A Tour of Raptor - 1:1 Classic Tour of Go Interactive Engine

let audioCtx = null;
let canvasCtx = null;
let activeCanvas = null;

// Browser Bridge for WebAssembly builtins (Canvas, DOM, WebAudio)
window.raptorBridge = {
  initCanvas: function(canvasId, width, height) {
    const canvas = document.getElementById(canvasId) || document.getElementById('wasmCanvas');
    if (!canvas) return;
    activeCanvas = canvas;
    if (width) canvas.width = width;
    if (height) canvas.height = height;
    canvasCtx = canvas.getContext('2d');
    switchToTab('tabCanvas', 'canvasView');
  },

  clearCanvas: function(color) {
    if (!canvasCtx || !activeCanvas) window.raptorBridge.initCanvas('wasmCanvas', 640, 380);
    if (!canvasCtx) return;
    canvasCtx.fillStyle = color || '#ffffff';
    canvasCtx.fillRect(0, 0, activeCanvas.width, activeCanvas.height);
  },

  drawRect: function(x, y, w, h, color, fill) {
    if (!canvasCtx) window.raptorBridge.initCanvas('wasmCanvas', 640, 380);
    if (!canvasCtx) return;
    if (fill) {
      canvasCtx.fillStyle = color || '#375eab';
      canvasCtx.fillRect(x, y, w, h);
    } else {
      canvasCtx.strokeStyle = color || '#375eab';
      canvasCtx.lineWidth = 2;
      canvasCtx.strokeRect(x, y, w, h);
    }
  },

  drawCircle: function(x, y, r, color, fill) {
    if (!canvasCtx) window.raptorBridge.initCanvas('wasmCanvas', 640, 380);
    if (!canvasCtx) return;
    canvasCtx.beginPath();
    canvasCtx.arc(x, y, r, 0, Math.PI * 2);
    if (fill) {
      canvasCtx.fillStyle = color || '#00add8';
      canvasCtx.fill();
    } else {
      canvasCtx.strokeStyle = color || '#00add8';
      canvasCtx.lineWidth = 2;
      canvasCtx.stroke();
    }
  },

  drawLine: function(x1, y1, x2, y2, color, lineWidth) {
    if (!canvasCtx) window.raptorBridge.initCanvas('wasmCanvas', 640, 380);
    if (!canvasCtx) return;
    canvasCtx.beginPath();
    canvasCtx.moveTo(x1, y1);
    canvasCtx.lineTo(x2, y2);
    canvasCtx.strokeStyle = color || '#f59e0b';
    canvasCtx.lineWidth = lineWidth || 1.5;
    canvasCtx.stroke();
  },

  drawText: function(text, x, y, size, color) {
    if (!canvasCtx) window.raptorBridge.initCanvas('wasmCanvas', 640, 380);
    if (!canvasCtx) return;
    canvasCtx.font = `${size || 14}px 'Fira Code', monospace`;
    canvasCtx.fillStyle = color || '#ffffff';
    canvasCtx.fillText(text, x, y);
  },

  initAudio: function() {
    if (!audioCtx) {
      const AudioContext = window.AudioContext || window.webkitAudioContext;
      audioCtx = new AudioContext();
    }
    if (audioCtx.state === 'suspended') {
      audioCtx.resume();
    }
  },

  playTone: function(frequency, durationSec, waveType) {
    window.raptorBridge.initAudio();
    if (!audioCtx) return;

    const osc = audioCtx.createOscillator();
    const gain = audioCtx.createGain();

    osc.type = waveType || 'sine';
    osc.frequency.setValueAtTime(frequency || 440, audioCtx.currentTime);

    gain.gain.setValueAtTime(0.15, audioCtx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.0001, audioCtx.currentTime + (durationSec || 0.25));

    osc.connect(gain);
    gain.connect(audioCtx.destination);

    osc.start();
    osc.stop(audioCtx.currentTime + (durationSec || 0.25));
  },

  playMelody: function(frequencies, durations) {
    window.raptorBridge.initAudio();
    if (!audioCtx || !Array.isArray(frequencies)) return;

    let timeOffset = 0;
    frequencies.forEach((freq, idx) => {
      const dur = (durations && durations[idx]) ? durations[idx] : 0.2;
      setTimeout(() => {
        window.raptorBridge.playTone(freq, dur, 'triangle');
      }, timeOffset * 1000);
      timeOffset += dur;
    });
  }
};

// 8 Pure-Wasm Tour Lessons
const TOUR_LESSONS = [
  {
    title: "Hello, Raptor",
    desc: `
      <p>Welcome to a tour of the <strong>Raptor</strong> programming language.</p>
      <p>Raptor is a high-performance procedural execution platform and dynamic language (Perl 5 subset of Raku without OO overhead). Variables use standard sigils (<code>$</code>, <code>@</code>, <code>%</code>).</p>
      <p>The tour is divided into a list of modules that you can access by clicking on the dropdown on the top right.</p>
      <p>Throughout the tour you will find a series of slides and exercises for you to complete.</p>
      <p>You can navigate through them using:</p>
      <p class="tour-indent-p">
        <span class="tour-blue-link" id="linkPrev">&quot;previous&quot;</span> to go to the previous page,<br>
        <span class="tour-blue-link" id="linkNext">&quot;next&quot;</span> to go to the next page.
      </p>
      <p>The tour is interactive. Click the <strong>Run</strong> button now (or press <kbd>Shift + Enter</kbd>) to compile and run the program in WebAssembly. The result is displayed below the code.</p>
      <p>When you click on <strong>Format</strong> (shortcut: <kbd>Ctrl + Enter</kbd>), code indentation is aligned.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Dynamic Variables & Sigils
my $name = "Raptor";
my @primes = [2, 3, 5, 7, 11, 13];
my %config = { "mode" => "WebAssembly", "speed" => "Instant" };

say "Welcome to $name (Mode: " ~ %config{"mode"} ~ ")!";

# 2. Advanced Operators
my $power = 2 ** 10;
say "2 ** 10 = ", $power;

my $divisible = 100 %% 4;
say "100 is divisible by 4? ", $divisible;

my $fallback = nil // "Default Fallback Value";
say "Defined-or check: ", $fallback;

my $min_val = 24 min 42;
my $max_val = 24 max 42;
say "min(24, 42) = ", $min_val, " | max(24, 42) = ", $max_val;

# 3. String & List Repetition
say "Repeat string: ", "Cyber " x 3;
say "Repeat array:  ", [1, 2] xx 3;
`
  },

  {
    title: "Dynamic Subsets & Predicates",
    desc: `
      <p>In Raptor, you can define named dynamic refinement types using <code>subset</code> and <code>where</code> boolean predicates.</p>
      <p>Subroutines can dispatch polymorphically based on runtime value predicates without static class hierarchies.</p>
      <p>Notice how <code>classify()</code> dispatches between <code>Even</code> and <code>Odd</code>, and how <code>fib()</code> uses predicate pattern matching directly in its parameter signature.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Define Dynamic Refinement Types
subset Even of Int where { $_ % 2 == 0 };
subset Odd of Int where { $_ % 2 != 0 };
subset Positive of Int where { $_ > 0 };

# 2. Multiple Dispatch based on Predicate Matches
multi sub classify(Even $n) {
    say "-> $n is an Even Integer";
}

multi sub classify(Odd $n) {
    say "-> $n is an Odd Integer";
}

# 3. Pattern Matching in Function Signatures (Predicate Recursion)
multi sub fib(Int $n where { $_ <= 1 }) {
    return $n;
}

multi sub fib(Int $n) {
    return fib($n - 1) + fib($n - 2);
}

say "--- Classification Test ---";
classify(42);
classify(17);

say "--- Predicate Fibonacci ---";
say "fib(0) = ", fib(0);
say "fib(1) = ", fib(1);
say "fib(8) = ", fib(8);
`
  },

  {
    title: "C-ABI Structs, Closures & Overloading",
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
    title: "Autothreading Quantum Junctions",
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
    title: "Signature Destructuring & Fast JSON",
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

# 2. Hash Parameter Destructuring
sub greet_user(:{:$name, :$role = "Developer", :$level = 1}) {
    say "User: $name | Role: $role | Level: $level";
}

say "--- Destructuring ---";
inspect_list([100, 200, 300, 400]);
greet_user({ name => "Ada Lovelace", role => "Pioneer", level => 99 });

# 3. Fast JSON Serialization & Parsing
my %telemetry = {
    "system" => "Raptor Wasm",
    "timestamp" => time(),
    "active" => true,
    "metrics" => [12.5, 45.8, 99.1]
};

my $json_payload = to_json(%telemetry);
say "--- Generated JSON ---";
say $json_payload;

my $parsed = from_json($json_payload);
say "Parsed system: ", $parsed{"system"};
`
  },

  {
    title: "HTML5 Canvas 2D Graphics",
    desc: `
      <p>Raptor scripts running in WebAssembly can render hardware-accelerated 2D graphics directly to the HTML5 Canvas viewport using <code>canvas_init</code>, <code>canvas_draw_rect</code>, <code>canvas_draw_circle</code>, and <code>canvas_draw_line</code>.</p>
      <p>Click <strong>Run</strong> and observe the <strong>Canvas 2D</strong> tab.</p>
    `,
    defaultTab: "tabCanvas",
    defaultView: "canvasView",
    code: `# 1. Initialize Canvas Viewport (640x380)
canvas_init("wasmCanvas", 640, 380);
canvas_clear("#ffffff");

# 2. Draw Frame and Title
canvas_draw_rect(10, 10, 620, 360, "#d0dbe5", false);
canvas_draw_text("RAPTOR WebAssembly 2D Canvas Engine", 30, 45, 16, "#007d9c");

# 3. Particle Starfield Structure
struct StarNode {
    num64 $x;
    num64 $y;
    num64 $radius;
    Str   $color;
}

my @nodes = [];
my @palette = ["#007d9c", "#00add8", "#f59e0b", "#8b5cf6", "#e11d48", "#10b981"];

for 1..10 -> $i {
    my $star = StarNode.new();
    $star.x = 60 + ($i * 50.0);
    $star.y = 120 + (($i * 43) % 170);
    $star.radius = 5.0 + (($i * 2) % 8);
    $star.color = @palette[$i % 6];
    @nodes.push($star);
}

# Connect nodes with constellation lines
for 0..8 -> $idx {
    my $s1 = @nodes[$idx];
    my $s2 = @nodes[$idx + 1];
    canvas_draw_line($s1.x, $s1.y, $s2.x, $s2.y, "#cbd5e1", 2.0);
}

# Draw glowing nodes
for @nodes -> $n {
    canvas_draw_circle($n.x, $n.y, $n.radius + 4.0, $n.color, false);
    canvas_draw_circle($n.x, $n.y, $n.radius, $n.color, true);
}

say "Canvas rendered with 10 interactive particle nodes!";
`
  },

  {
    title: "WebAudio API Sound Synthesizer",
    desc: `
      <p>Generate audio waveforms, musical arpeggios, and synthesizers in real-time through the browser's WebAudio API directly from WebAssembly.</p>
      <p>Click <strong>Run</strong> to listen to the generated musical sequence.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Initialize WebAudio Context
audio_init();

say "--- Synthesizing WebAudio Arpeggio ---";

# Frequencies in Hz: C4, E4, G4, B4, C5 (Major 7th Arpeggio)
my @melody = [261.63, 329.63, 392.00, 493.88, 523.25];
my @durations = [0.15, 0.15, 0.15, 0.15, 0.35];

# Play musical sequence
audio_play_melody(@melody, @durations);

say "WebAudio Arpeggio playing via browser WebAudio API!";
`
  },

  {
    title: "Full-Stack WebAssembly Application",
    desc: `
      <p>Combine HTML5 Canvas 2D graphics, WebAudio sound synthesis, live DOM manipulation, and JSON telemetry into a complete interactive application running 100% client-side in pure WebAssembly.</p>
    `,
    defaultTab: "tabCanvas",
    defaultView: "canvasView",
    code: `# === Full-Stack Raptor WebAssembly Application ===

# 1. Canvas 2D Graphics Viewport
canvas_init("wasmCanvas", 640, 380);
canvas_clear("#ffffff");
canvas_draw_rect(10, 10, 620, 360, "#d0dbe5", false);
canvas_draw_text("RAPTOR Full-Stack Web App Engine", 30, 45, 16, "#007d9c");

struct AppNode {
    num64 $x;
    num64 $y;
    num64 $r;
    Str   $color;
}

my @nodes = [];
my @colors = ["#007d9c", "#00add8", "#f59e0b", "#8b5cf6", "#e11d48", "#10b981"];

for 1..10 -> $i {
    my $node = AppNode.new();
    $node.x = 55 + ($i * 52.0);
    $node.y = 130 + (($i * 37) % 170);
    $node.r = 6.0 + (($i * 3) % 10);
    $node.color = @colors[$i % 6];
    @nodes.push($node);
}

for 0..8 -> $idx {
    my $n1 = @nodes[$idx];
    my $n2 = @nodes[$idx + 1];
    canvas_draw_line($n1.x, $n1.y, $n2.x, $n2.y, "#cbd5e1", 2.0);
}

for @nodes -> $n {
    canvas_draw_circle($n.x, $n.y, $n.r + 4.0, $n.color, false);
    canvas_draw_circle($n.x, $n.y, $n.r, $n.color, true);
}

# 2. JSON Telemetry
my %telemetry = {
    "engine" => "Raptor WebAssembly",
    "timestamp" => time(),
    "node_count" => @nodes.elems(),
    "subsystems" => ["Canvas2D", "WebAudio", "DOM", "JSON"]
};
my $json_str = to_json(%telemetry);
say "--- Generated Telemetry ---";
say $json_str;

# 3. Dynamic DOM Manipulation
dom_set_html("#wasmDomContainer", 
    "<div style='color:#375eab; font-weight:bold; font-size:1.15rem; margin-bottom:0.5rem;'>Raptor Web Control Panel</div>" ~
    "<p style='color:#666666; font-size:0.9rem; margin-bottom:0.5rem;'>Simulated " ~ @nodes.elems() ~ " nodes in pure WebAssembly.</p>" ~
    "<pre style='background:#f1f5f9; padding:0.75rem; border-radius:4px; font-family:monospace; color:#007d9c; font-size:0.82rem; border:1px solid #e2e8f0;'>" ~ $json_str ~ "</pre>"
);

# 4. WebAudio Synthesis
my @melody = [261.63, 329.63, 392.00, 493.88, 523.25];
my @durations = [0.15, 0.15, 0.15, 0.15, 0.35];
audio_play_melody(@melody, @durations);

say "Full-stack application executed: Canvas, DOM, Audio, and JSON all active!";
`
  }
];

let currentLessonIndex = 0;
let wasmReady = false;
let replHistory = [];
let historyIndex = -1;

// Elements
const statusDot = document.getElementById('statusDot');
const statusText = document.getElementById('statusText');
const btnRun = document.getElementById('btnRun');
const btnResetCode = document.getElementById('btnResetCode');
const btnFormatCode = document.getElementById('btnFormatCode');
const btnPrevLesson = document.getElementById('btnPrevLesson');
const btnNextLesson = document.getElementById('btnNextLesson');
const lessonSelect = document.getElementById('lessonSelect');
const pageIndicator = document.getElementById('pageIndicator');
const lineNumbers = document.getElementById('lineNumbers');

const lessonTitle = document.getElementById('lessonTitle');
const lessonDesc = document.getElementById('lessonDesc');
const codeEditor = document.getElementById('codeEditor');

const tabConsole = document.getElementById('tabConsole');
const tabCanvas = document.getElementById('tabCanvas');
const tabDom = document.getElementById('tabDom');
const tabPod = document.getElementById('tabPod');

const consoleView = document.getElementById('consoleView');
const canvasView = document.getElementById('canvasView');
const domView = document.getElementById('domView');
const podView = document.getElementById('podView');

const consoleTerminal = document.getElementById('consoleTerminal');
const replInput = document.getElementById('replInput');
const btnClearConsole = document.getElementById('btnClearConsole');
const podPreview = document.getElementById('podPreview');
const btnWeave = document.getElementById('btnWeave');
const btnTangle = document.getElementById('btnTangle');

// Update line numbers gutter
function updateLineNumbers() {
  if (!codeEditor || !lineNumbers) return;
  const lines = codeEditor.value.split('\n').length;
  let nums = '';
  for (let i = 1; i <= Math.max(lines, 20); i++) {
    nums += i + '<br>';
  }
  lineNumbers.innerHTML = nums;
}

codeEditor.addEventListener('input', updateLineNumbers);

// Tab Switching
function switchToTab(tabId, viewId) {
  [tabConsole, tabCanvas, tabDom, tabPod].forEach(t => t && t.classList.remove('active'));
  [consoleView, canvasView, domView, podView].forEach(v => v && v.classList.remove('active'));

  const targetTab = document.getElementById(tabId);
  const targetView = document.getElementById(viewId);
  if (targetTab) targetTab.classList.add('active');
  if (targetView) targetView.classList.add('active');
}

tabConsole.addEventListener('click', () => switchToTab('tabConsole', 'consoleView'));
tabCanvas.addEventListener('click', () => switchToTab('tabCanvas', 'canvasView'));
tabDom.addEventListener('click', () => switchToTab('tabDom', 'domView'));
tabPod.addEventListener('click', () => {
  switchToTab('tabPod', 'podView');
  renderWeave();
});

// Load Tour Lesson
function loadLesson(index) {
  if (index < 0) index = 0;
  if (index >= TOUR_LESSONS.length) index = TOUR_LESSONS.length - 1;

  currentLessonIndex = index;
  lessonSelect.value = index;
  pageIndicator.textContent = `${index + 1} / ${TOUR_LESSONS.length}`;

  const lesson = TOUR_LESSONS[index];
  lessonTitle.textContent = lesson.title;
  lessonDesc.innerHTML = lesson.desc;
  codeEditor.value = lesson.code;

  // Re-hook inline link events
  const linkPrev = document.getElementById('linkPrev');
  const linkNext = document.getElementById('linkNext');
  if (linkPrev) linkPrev.addEventListener('click', () => loadLesson(currentLessonIndex - 1));
  if (linkNext) linkNext.addEventListener('click', () => loadLesson(currentLessonIndex + 1));

  updateLineNumbers();
  switchToTab(lesson.defaultTab, lesson.defaultView);
}

lessonSelect.addEventListener('change', (e) => loadLesson(parseInt(e.target.value, 10)));
btnPrevLesson.addEventListener('click', () => loadLesson(currentLessonIndex - 1));
btnNextLesson.addEventListener('click', () => loadLesson(currentLessonIndex + 1));
btnResetCode.addEventListener('click', () => loadLesson(currentLessonIndex));

btnFormatCode.addEventListener('click', () => {
  // Simple code indentation alignment
  const lines = codeEditor.value.split('\n');
  codeEditor.value = lines.map(l => l.replace(/^\t+/, '    ')).join('\n');
  updateLineNumbers();
});

// Initialize WebAssembly Runtime
async function initWasm() {
  try {
    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(
      fetch('raptor.wasm'),
      go.importObject
    );
    go.run(result.instance);

    wasmReady = true;
    statusDot.classList.add('ready');
    statusText.textContent = window.raptorVersion ? window.raptorVersion() : "Raptor Ready";
  } catch (err) {
    statusDot.style.background = '#dc2626';
    statusText.textContent = 'Wasm Load Error';
    appendTerminalEntry(null, null, `WebAssembly initialization error: ${err.message}`, true);
  }
}

// Terminal Output
function appendTerminalEntry(inputCode, resultVal, outputText, isError = false) {
  const entry = document.createElement('div');
  entry.className = 'terminal-entry';

  if (inputCode) {
    const inLine = document.createElement('div');
    inLine.className = 'terminal-input-line';
    inLine.textContent = `> ${inputCode}`;
    entry.appendChild(inLine);
  }

  if (outputText && outputText.trim().length > 0) {
    const outLine = document.createElement('div');
    outLine.className = 'terminal-output-line';
    outLine.textContent = outputText;
    entry.appendChild(outLine);
  }

  if (isError) {
    const errLine = document.createElement('div');
    errLine.className = 'terminal-error-line';
    errLine.textContent = outputText || 'Evaluation Error';
    entry.appendChild(errLine);
  } else if (resultVal !== undefined && resultVal !== null && resultVal !== "Nil" && resultVal !== "") {
    const resLine = document.createElement('div');
    resLine.className = 'terminal-result-line';
    resLine.textContent = `=> ${resultVal}`;
    entry.appendChild(resLine);
  }

  consoleTerminal.appendChild(entry);
  consoleTerminal.scrollTop = consoleTerminal.scrollHeight;
}

// Execute Code
function executeCode(sourceCode) {
  if (!wasmReady) {
    appendTerminalEntry(sourceCode, null, "WebAssembly engine is loading. Please wait...", true);
    return;
  }

  const res = window.raptorEval(sourceCode);
  if (res.error) {
    appendTerminalEntry(sourceCode, null, res.error, true);
  } else {
    appendTerminalEntry(sourceCode, res.result, res.output, false);
  }
}

btnRun.addEventListener('click', () => executeCode(codeEditor.value));

btnClearConsole.addEventListener('click', () => {
  consoleTerminal.innerHTML = '';
});

// REPL input prompt
replInput.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    const text = replInput.value.trim();
    if (text.length > 0) {
      replHistory.push(text);
      historyIndex = replHistory.length;
      executeCode(text);
      replInput.value = '';
    }
  } else if (e.key === 'ArrowUp') {
    if (historyIndex > 0) {
      historyIndex--;
      replInput.value = replHistory[historyIndex];
    }
  } else if (e.key === 'ArrowDown') {
    if (historyIndex < replHistory.length - 1) {
      historyIndex++;
      replInput.value = replHistory[historyIndex];
    } else {
      historyIndex = replHistory.length;
      replInput.value = '';
    }
  }
});

// Global keyboard shortcuts (Shift+Enter, Ctrl+Enter, PageUp, PageDown)
window.addEventListener('keydown', (e) => {
  if ((e.shiftKey || e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault();
    executeCode(codeEditor.value);
  } else if (e.key === 'PageUp') {
    e.preventDefault();
    loadLesson(currentLessonIndex - 1);
  } else if (e.key === 'PageDown') {
    e.preventDefault();
    loadLesson(currentLessonIndex + 1);
  }
});

// PodLit Weave / Tangle
function renderWeave() {
  if (!wasmReady) return;
  const podSource = codeEditor.value;
  const res = window.raptorWeave(podSource);
  if (res.error) {
    podPreview.innerHTML = `<div class="terminal-error-line">${res.error}</div>`;
  } else {
    let html = res.markdown
      .replace(/^# (.*$)/gim, '<h1>$1</h1>')
      .replace(/^## (.*$)/gim, '<h2>$1</h2>')
      .replace(/^### (.*$)/gim, '<h3>$1</h3>')
      .replace(/```([\s\S]*?)```/gim, '<pre><code>$1</code></pre>');
    podPreview.innerHTML = html;
  }
}

btnWeave.addEventListener('click', renderWeave);
btnTangle.addEventListener('click', () => {
  if (!wasmReady) return;
  const podSource = codeEditor.value;
  const res = window.raptorTangle(podSource);
  if (res.error) {
    podPreview.innerHTML = `<div class="terminal-error-line">${res.error}</div>`;
  } else {
    let out = '<h3>Tangled Files:</h3>';
    for (const [fn, content] of Object.entries(res.files)) {
      out += `<h4>📄 ${fn}</h4><pre><code>${content.replace(/</g, '&lt;').replace(/>/g, '&gt;')}</code></pre>`;
    }
    podPreview.innerHTML = out;
  }
});

// Load first lesson on start
loadLesson(0);

// Initialize Wasm
initWasm();
