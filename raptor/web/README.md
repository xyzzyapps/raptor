# Raptor WebAssembly Architecture & Browser Subsystems

Raptor provides first-class support for client-side execution in all modern web browsers via WebAssembly (`WASM`). The runtime compiles directly into standalone `raptor.wasm` with full access to lexical scopes, multiple dispatch, refinement types, literate PodLit weaves/tangles, and reactive browser subsystems.

---

## 1. Building the WebAssembly Binary

To build `raptor.wasm` from the Go runtime:

```bash
# Set WebAssembly target environment variables
$env:GOOS = "js"
$env:GOARCH = "wasm"

# Compile standalone WebAssembly binary
go build -o web/raptor.wasm ./cmd/raptor
```

The Go WebAssembly support library `wasm_exec.js` is included in `web/wasm_exec.js` (from `$(go env GOROOT)/misc/wasm/wasm_exec.js`).

---

## 2. Serving Locally

Start the built-in HTTP dev server from the repository root:

```bash
raptor serve 8080
# Or using Go:
go run ./cmd/raptor serve 8080
```

Open `http://localhost:8080/web/` to launch the **Tour of Raptor** interactive environment.

---

## 3. JavaScript Bridge API (`window.raptor*`)

When `raptor.wasm` is instantiated in the browser, it exposes synchronous and async bridge methods on `window`:

| Exported JavaScript API | Signature | Description |
| :--- | :--- | :--- |
| `window.raptorEval(code)` | `(code: string) => string` | Evaluates pure Raptor code and captures standard output stream. |
| `window.raptorWeave(pod)` | `(podText: string) => string` | Weaves PodLit literate document into structured GitHub Flavored Markdown. |
| `window.raptorTangle(pod, target)` | `(podText: string, targetFile: string) => string` | Extracts executable source code chunk targeting `targetFile`. |
| `window.raptorStitch(pod, target, code)` | `(podText: string, targetFile: string, updatedCode: string) => string` | Synchronizes modified source code back into PodLit documentation. |

### Example JavaScript Invocation

```javascript
// Run Raptor script in browser
const output = window.raptorEval(`
    sub fib($n) {
        if $n <= 1 { return $n; }
        return fib($n - 1) + fib($n - 2);
    }
    say "Fib(20) = " ~ fib(20);
`);
console.log(output); // Output: "Fib(20) = 6765\n"
```

---

## 4. Hardware Browser Subsystems (`raptorBridge`)

The runtime interacts with browser APIs through `window.raptorBridge`:

### A. HTML5 Canvas 2D Graphics
- `canvasGetContext(canvasId, width, height)`: Attaches a 2D rendering context.
- `canvasSetFillStyle(ctxId, color)` / `canvasSetStrokeStyle(ctxId, color)`
- `canvasFillRect(ctxId, x, y, w, h)` / `canvasStrokeRect(ctxId, x, y, w, h)`
- `canvasBeginPath(ctxId)`, `canvasArc(ctxId, x, y, r, sa, ea)`, `canvasFill(ctxId)`
- `canvasDrawText(ctxId, text, x, y, font, color)`

### B. Hardware WebGL 3D
- `webglInit(canvasId)`: Initializes hardware 3D context.
- `webglCreateShader(glId, type, source)`: Compiles GLSL vertex and fragment shaders.
- `webglCreateProgram(glId, vsId, fsId)`: Links shader pipeline program.
- `webglCreateBuffer(glId, dataArray)`: Uploads vertex VBO data to GPU memory.
- `webglDrawArrays(glId, mode, first, count)`: Issues GPU draw calls.

### C. WebAudio Digital Signal Processing
- `audioInit()`: Initializes WebAudio AudioContext.
- `audioCreateOscillator(type, frequency)`: Spawns synthesized oscillators (sine, square, sawtooth, triangle).
- `audioCreateGain(initialVolume)`: Configures gain nodes for volume envelopes.
- `audioStartOscillator(oscId)` / `audioStopOscillator(oscId)`

### D. DOM Interop
- `domQuerySelector(selector)`: Selects elements from the HTML DOM tree.
- `domSetTextContent(elementId, text)`: Updates element text.
- `domAddEventListener(elementId, eventName, callback)`: Binds event listeners to user actions.

---

## 5. Security & Sandboxing

In the WebAssembly environment:
- File system access is emulated in-memory.
- Native C FFI bindings (`is native`) gracefully return stubbed values or route to browser WebGL/WebAudio APIs.
- Backtick execution is sandboxed and does not execute arbitrary host shell commands in the browser context.
