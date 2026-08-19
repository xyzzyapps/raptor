// ==============================================================================
// A Tour of Raptor - 1:1 Classic Tour of Go Interactive WebAssembly Engine
// ==============================================================================

let audioCtx = null;
window.audioCtx = null;
let canvasContexts = [];
let audioNodeSeq = 1;
let audioNodes = {}; // id -> { kind, node, ctx, extra }

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

  // --- Low-Level WebAudio DSP (1:1 AudioContext / AudioNode wrappers) ---
  _audioAlloc: function(kind, node, ctx, extra) {
    const id = audioNodeSeq++;
    audioNodes[id] = { kind, node, ctx, extra: extra || null };
    return id;
  },
  _audioEntry: function(id) {
    return audioNodes[id] || null;
  },
  _audioNode: function(id) {
    const e = audioNodes[id];
    return e ? e.node : null;
  },
  _audioCtx: function(id) {
    const e = audioNodes[id];
    if (e && e.kind === 'context') return e.node;
    if (e && e.ctx) return e.ctx;
    return audioCtx;
  },
  _audioResume: function(ctx) {
    if (ctx && ctx.state === 'suspended') {
      ctx.resume().catch(() => {});
    }
  },

  initAudio: function() {
    if (!audioCtx) {
      const AC = window.AudioContext || window.webkitAudioContext;
      audioCtx = new AC();
      window.audioCtx = audioCtx;
    }
    this._audioResume(audioCtx);
    console.log("[raptorBridge] initAudio() state:", audioCtx.state);
  },

  playTone: function(frequency, durationSec, waveType) {
    // Kept as a thin timeline helper; tour lesson 13 builds the graph in Raptor.
    this.initAudio();
    if (!audioCtx) return;
    const t0 = audioCtx.currentTime;
    const osc = audioCtx.createOscillator();
    const gain = audioCtx.createGain();
    osc.type = waveType || 'triangle';
    osc.frequency.setValueAtTime(frequency || 440, t0);
    gain.gain.setValueAtTime(0.18, t0);
    gain.gain.exponentialRampToValueAtTime(0.0001, t0 + (durationSec || 0.25));
    osc.connect(gain);
    gain.connect(audioCtx.destination);
    osc.start(t0);
    osc.stop(t0 + (durationSec || 0.25));
  },

  audioContextCreate: function() {
    const AC = window.AudioContext || window.webkitAudioContext;
    const ctx = new AC();
    this._audioResume(ctx);
    audioCtx = ctx;
    window.audioCtx = ctx;
    const destId = this._audioAlloc('destination', ctx.destination, ctx);
    const ctxId = this._audioAlloc('context', ctx, ctx, { destId });
    console.log("[raptorBridge] AudioContext", ctx.state, "sampleRate", ctx.sampleRate, "id", ctxId);
    return ctxId;
  },

  audioGetCurrentTime: function(ctxId) {
    const ctx = this._audioCtx(ctxId);
    return ctx ? ctx.currentTime : 0.0;
  },

  audioSampleRate: function(ctxId) {
    const ctx = this._audioCtx(ctxId);
    return ctx ? ctx.sampleRate : 44100;
  },

  audioDestination: function(ctxId) {
    const e = this._audioEntry(ctxId);
    if (e && e.extra && e.extra.destId) return e.extra.destId;
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    return this._audioAlloc('destination', ctx.destination, ctx);
  },

  audioCreateOscillator: function(ctxId) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    return this._audioAlloc('oscillator', ctx.createOscillator(), ctx);
  },
  audioCreateGain: function(ctxId) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    return this._audioAlloc('gain', ctx.createGain(), ctx);
  },
  audioCreateBiquadFilter: function(ctxId, filterType) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    const filter = ctx.createBiquadFilter();
    filter.type = filterType || 'lowpass';
    return this._audioAlloc('filter', filter, ctx);
  },
  audioCreateCompressor: function(ctxId) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    return this._audioAlloc('compressor', ctx.createDynamicsCompressor(), ctx);
  },
  audioCreateDelay: function(ctxId, maxDelay) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    return this._audioAlloc('delay', ctx.createDelay(maxDelay || 1.0), ctx);
  },
  audioCreatePanner: function(ctxId) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    return this._audioAlloc('panner', ctx.createStereoPanner(), ctx);
  },
  audioCreateAnalyser: function(ctxId) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    const an = ctx.createAnalyser();
    an.fftSize = 256;
    return this._audioAlloc('analyser', an, ctx);
  },
  audioCreateBuffer: function(ctxId, channels, length, sampleRate) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    const buf = ctx.createBuffer(channels || 1, length || ctx.sampleRate, sampleRate || ctx.sampleRate);
    return this._audioAlloc('buffer', buf, ctx);
  },
  audioCreateBufferSource: function(ctxId) {
    const ctx = this._audioCtx(ctxId);
    if (!ctx) return 0;
    return this._audioAlloc('source', ctx.createBufferSource(), ctx);
  },

  audioConnect: function(srcId, dstId) {
    const src = this._audioNode(srcId);
    const dst = this._audioNode(dstId);
    if (src && dst && src.connect) {
      try { src.connect(dst); } catch (err) { console.warn("audioConnect", err); }
    }
  },
  audioConnectParam: function(srcId, dstId, paramName) {
    const src = this._audioNode(srcId);
    const dst = this._audioNode(dstId);
    if (src && dst && dst[paramName]) {
      try { src.connect(dst[paramName]); } catch (err) { console.warn("audioConnectParam", err); }
    }
  },
  audioConnectDestination: function(srcId, ctxId) {
    const src = this._audioNode(srcId);
    const ctx = this._audioCtx(ctxId);
    if (src && ctx) {
      try { src.connect(ctx.destination); } catch (err) { console.warn("audioConnectDestination", err); }
    }
  },
  audioDisconnect: function(srcId) {
    const src = this._audioNode(srcId);
    if (src && src.disconnect) {
      try { src.disconnect(); } catch (err) { console.warn("audioDisconnect", err); }
    }
  },

  audioSetOscType: function(oscId, waveType) {
    const osc = this._audioNode(oscId);
    if (osc) osc.type = waveType || 'sine';
  },
  audioSetFrequency: function(oscId, freq, timeOffset) {
    const osc = this._audioNode(oscId);
    if (osc && osc.frequency) osc.frequency.setValueAtTime(freq, timeOffset || 0.0);
  },
  audioFreqRamp: function(oscId, freq, endTime) {
    const osc = this._audioNode(oscId);
    if (osc && osc.frequency) osc.frequency.exponentialRampToValueAtTime(Math.max(freq, 1), endTime);
  },
  audioSetDetune: function(oscId, cents, timeOffset) {
    const osc = this._audioNode(oscId);
    if (osc && osc.detune) osc.detune.setValueAtTime(cents, timeOffset || 0.0);
  },
  audioSetGain: function(gainId, gainVal, timeOffset) {
    const gain = this._audioNode(gainId);
    if (gain && gain.gain) gain.gain.setValueAtTime(gainVal, timeOffset || 0.0);
  },
  audioGainRampExp: function(gainId, targetVal, endTime) {
    const gain = this._audioNode(gainId);
    if (gain && gain.gain) {
      gain.gain.exponentialRampToValueAtTime(Math.max(targetVal, 0.00001), endTime);
    }
  },
  audioGainRampLinear: function(gainId, targetVal, endTime) {
    const gain = this._audioNode(gainId);
    if (gain && gain.gain) gain.gain.linearRampToValueAtTime(targetVal, endTime);
  },
  audioSetFilterFreq: function(filterId, freq, timeOffset) {
    const filter = this._audioNode(filterId);
    if (filter && filter.frequency) filter.frequency.setValueAtTime(freq, timeOffset || 0.0);
  },
  audioSetFilterQ: function(filterId, q, timeOffset) {
    const filter = this._audioNode(filterId);
    if (filter && filter.Q) filter.Q.setValueAtTime(q, timeOffset || 0.0);
  },
  audioSetCompressor: function(id, thresh, knee, ratio, attack, release) {
    const c = this._audioNode(id);
    if (!c || !c.threshold) return;
    const t = c.context ? c.context.currentTime : 0;
    c.threshold.setValueAtTime(thresh, t);
    c.knee.setValueAtTime(knee, t);
    c.ratio.setValueAtTime(ratio, t);
    c.attack.setValueAtTime(attack, t);
    c.release.setValueAtTime(release, t);
  },
  audioSetDelayTime: function(id, sec, timeOffset) {
    const d = this._audioNode(id);
    if (d && d.delayTime) d.delayTime.setValueAtTime(sec, timeOffset || 0.0);
  },
  audioSetPan: function(id, pan, timeOffset) {
    const p = this._audioNode(id);
    if (p && p.pan) p.pan.setValueAtTime(pan, timeOffset || 0.0);
  },
  audioSetFftSize: function(id, n) {
    const a = this._audioNode(id);
    if (a) a.fftSize = n || 256;
  },
  audioGetSpectrum: function(id) {
    const a = this._audioNode(id);
    if (!a || !a.getByteFrequencyData) return [];
    const buf = new Uint8Array(a.frequencyBinCount);
    a.getByteFrequencyData(buf);
    return Array.from(buf);
  },
  audioBufferFillSine: function(id, freq) {
    const buf = this._audioNode(id);
    if (!buf || !buf.getChannelData) return;
    const data = buf.getChannelData(0);
    const sr = buf.sampleRate;
    const f = freq || 440;
    for (let i = 0; i < data.length; i++) {
      data[i] = Math.sin(2 * Math.PI * f * i / sr) * 0.3;
    }
  },
  audioSourceSetBuffer: function(srcId, bufId) {
    const src = this._audioNode(srcId);
    const buf = this._audioNode(bufId);
    if (src && buf) src.buffer = buf;
  },
  audioSourceStart: function(srcId, startTime) {
    const src = this._audioNode(srcId);
    const e = this._audioEntry(srcId);
    if (e) this._audioResume(e.ctx);
    if (src && src.start) {
      try { src.start(startTime || 0.0); } catch (err) { console.warn("audioSourceStart", err); }
    }
  },
  audioOscStart: function(oscId, startTime) {
    const osc = this._audioNode(oscId);
    const e = this._audioEntry(oscId);
    if (e) this._audioResume(e.ctx);
    if (osc && osc.start) {
      try { osc.start(startTime || 0.0); } catch (err) { console.warn("audioOscStart", err); }
    }
  },
  audioOscStop: function(oscId, stopTime) {
    const osc = this._audioNode(oscId);
    if (osc && osc.stop) {
      try { osc.stop(stopTime || 0.0); } catch (err) { console.warn("audioOscStop", err); }
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
  },

  // --- WebGPU compute + logit bars (tiny LLM) ---
  webgpuReady: false,
  _gpu: null,

  webgpuInit: function(canvasId, width, height, silent) {
    const self = this;
    const canvas = document.getElementById(canvasId) || document.getElementById('wasmWebGPUCanvas');
    if (canvas) {
      if (width) canvas.width = width;
      if (height) canvas.height = height;
      if (!silent) switchToTab('tabWebGPU', 'webgpuView');
    }
    if (self._gpu && self._gpu.pending) return 1;
    if (!navigator.gpu) {
      console.warn("[raptorBridge] WebGPU not available");
      self.webgpuReady = false;
      return 0;
    }
    self._gpu = self._gpu || { pending: true };
    (async () => {
      try {
        const adapter = await navigator.gpu.requestAdapter();
        if (!adapter) {
          self.webgpuReady = false;
          self._gpu.pending = false;
          return;
        }
        const device = await adapter.requestDevice();
        const format = navigator.gpu.getPreferredCanvasFormat();
        let ctx = null;
        if (canvas) {
          ctx = canvas.getContext('webgpu');
          if (ctx) {
            ctx.configure({ device, format, alphaMode: 'opaque' });
          }
        }
        const matmulShader = device.createShaderModule({
          code: `
            struct Dims { m: u32, n: u32, k: u32, _pad: u32 }
            @group(0) @binding(0) var<storage, read> A: array<f32>;
            @group(0) @binding(1) var<storage, read> B: array<f32>;
            @group(0) @binding(2) var<storage, read_write> C: array<f32>;
            @group(0) @binding(3) var<uniform> dims: Dims;
            @compute @workgroup_size(8, 8)
            fn main(@builtin(global_invocation_id) gid: vec3u) {
              let row = gid.x;
              let col = gid.y;
              if (row >= dims.m || col >= dims.n) { return; }
              var acc = 0.0;
              for (var t = 0u; t < dims.k; t++) {
                acc += A[row * dims.k + t] * B[t * dims.n + col];
              }
              C[row * dims.n + col] = acc;
            }
          `
        });
        const pipeline = device.createComputePipeline({
          layout: 'auto',
          compute: { module: matmulShader, entryPoint: 'main' }
        });
        self._gpu = { device, canvas, ctx, format, pipeline, pending: false };
        self.webgpuReady = true;
        console.log("[raptorBridge] WebGPU ready", format);
      } catch (err) {
        console.warn("[raptorBridge] WebGPU init failed", err);
        self.webgpuReady = false;
        if (self._gpu) self._gpu.pending = false;
      }
    })();
    return 1;
  },

  webgpuAvailable: function() {
    return !!(this.webgpuReady && this._gpu && this._gpu.device);
  },

  webgpuMatmul: function(m, n, k, a, b) {
    const A = Array.isArray(a) ? a : Array.from(a || []);
    const B = Array.isArray(b) ? b : Array.from(b || []);
    return this._cpuMatmul ? this._cpuMatmul(m, n, k, A, B) : (function () {
      const C = new Array(m * n).fill(0);
      for (let i = 0; i < m; i++) for (let j = 0; j < n; j++) {
        let acc = 0; for (let t = 0; t < k; t++) acc += A[i * k + t] * B[t * n + j];
        C[i * n + j] = acc;
      }
      return C;
    })();
  },

  webgpuMatmulAsync: function(m, n, k, a, b, cb) {
    const self = this;
    const A = Array.isArray(a) ? a : Array.from(a || []);
    const B = Array.isArray(b) ? b : Array.from(b || []);
    const cpu = function () {
      const C = new Array(m * n).fill(0);
      for (let i = 0; i < m; i++) for (let j = 0; j < n; j++) {
        let acc = 0; for (let t = 0; t < k; t++) acc += A[i * k + t] * B[t * n + j];
        C[i * n + j] = acc;
      }
      return C;
    };
    if (!self.webgpuAvailable() || !self._webgpuMatmulRead) {
      cb(cpu());
      return;
    }
    self._webgpuMatmulRead(m, n, k, A, B).then(function (C) { cb(C); }).catch(function () { cb(cpu()); });
  },

  _webgpuMatmulRead: async function(m, n, k, A, B) {
    const device = this._gpu.device;
    const aBuf = device.createBuffer({ size: A.length * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST });
    const bBuf = device.createBuffer({ size: B.length * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST });
    const cBuf = device.createBuffer({ size: m * n * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC });
    const uBuf = device.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
    const rBuf = device.createBuffer({ size: m * n * 4, usage: GPUBufferUsage.MAP_READ | GPUBufferUsage.COPY_DST });
    device.queue.writeBuffer(aBuf, 0, new Float32Array(A));
    device.queue.writeBuffer(bBuf, 0, new Float32Array(B));
    device.queue.writeBuffer(uBuf, 0, new Uint32Array([m, n, k, 0]));
    const bind = device.createBindGroup({
      layout: this._gpu.pipeline.getBindGroupLayout(0),
      entries: [
        { binding: 0, resource: { buffer: aBuf } },
        { binding: 1, resource: { buffer: bBuf } },
        { binding: 2, resource: { buffer: cBuf } },
        { binding: 3, resource: { buffer: uBuf } }
      ]
    });
    const enc = device.createCommandEncoder();
    const pass = enc.beginComputePass();
    pass.setPipeline(this._gpu.pipeline);
    pass.setBindGroup(0, bind);
    pass.dispatchWorkgroups(Math.ceil(m / 8), Math.ceil(n / 8));
    pass.end();
    enc.copyBufferToBuffer(cBuf, 0, rBuf, 0, m * n * 4);
    device.queue.submit([enc.finish()]);
    await rBuf.mapAsync(GPUMapMode.READ);
    const copy = Array.from(new Float32Array(rBuf.getMappedRange().slice(0)));
    rBuf.unmap();
    return copy;
  },

  webgpuDrawLogits: function(logits, vocab) {
    const arr = Array.isArray(logits) ? logits : Array.from(logits || []);
    const canvas = (this._gpu && this._gpu.canvas) || document.getElementById('wasmWebGPUCanvas');
    if (!canvas) return;
    switchToTab('tabWebGPU', 'webgpuView');

    // Softmax + top-16
    let maxv = -Infinity;
    for (const v of arr) if (v > maxv) maxv = v;
    const probs = arr.map(v => Math.exp(v - maxv));
    let sum = 0;
    for (const p of probs) sum += p;
    const norm = probs.map(p => (sum ? p / sum : 0));
    const idx = norm.map((p, i) => i).sort((a, b) => norm[b] - norm[a]).slice(0, 16);

    if (this.webgpuAvailable() && this._gpu.ctx) {
      this._webgpuDrawBars(idx, norm, vocab || '');
      return;
    }
    if (this._gpu && this._gpu.ctx) {
      return;
    }
    const ctx2d = canvas.getContext('2d');
    if (!ctx2d) return;
    const w = canvas.width, h = canvas.height;
    ctx2d.fillStyle = '#0b1220';
    ctx2d.fillRect(0, 0, w, h);
    ctx2d.fillStyle = '#94a3b8';
    ctx2d.font = "14px 'Fira Code', monospace";
    ctx2d.fillText('tiny LLM next-char probs' + (this.webgpuReady ? ' (WebGPU)' : ' (2D fallback)'), 16, 24);
    const barW = Math.max(8, (w - 40) / idx.length - 6);
    idx.forEach((vi, i) => {
      const bh = norm[vi] * (h - 70);
      const x = 20 + i * (barW + 6);
      ctx2d.fillStyle = '#007d9c';
      ctx2d.fillRect(x, h - 28 - bh, barW, bh);
      ctx2d.fillStyle = '#e2e8f0';
      ctx2d.font = "12px 'Fira Code', monospace";
      const ch = (vocab && vocab[vi]) ? vocab[vi] : String(vi);
      ctx2d.fillText(ch === ' ' ? '␣' : ch, x, h - 10);
    });
  },

  _webgpuDrawBars: function(idx, norm, vocab) {
    const gpu = this._gpu;
    const w = gpu.canvas.width, h = gpu.canvas.height;
    const verts = [];
    const barW = 2 / idx.length * 0.7;
    idx.forEach((vi, i) => {
      const x0 = -0.92 + i * (1.84 / idx.length);
      const y0 = -0.75;
      const y1 = y0 + norm[vi] * 1.5;
      const x1 = x0 + barW;
      verts.push(x0, y0, x1, y0, x0, y1, x0, y1, x1, y0, x1, y1);
    });
    const device = gpu.device;
    const module = device.createShaderModule({
      code: `
        @vertex fn vs(@location(0) pos: vec2f) -> @builtin(position) vec4f {
          return vec4f(pos, 0.0, 1.0);
        }
        @fragment fn fs() -> @location(0) vec4f {
          return vec4f(0.0, 0.49, 0.61, 1.0);
        }
      `
    });
    const pipeline = device.createRenderPipeline({
      layout: 'auto',
      vertex: {
        module,
        entryPoint: 'vs',
        buffers: [{ arrayStride: 8, attributes: [{ shaderLocation: 0, offset: 0, format: 'float32x2' }] }]
      },
      fragment: { module, entryPoint: 'fs', targets: [{ format: gpu.format }] }
    });
    const vbo = device.createBuffer({
      size: verts.length * 4,
      usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST
    });
    device.queue.writeBuffer(vbo, 0, new Float32Array(verts));
    const enc = device.createCommandEncoder();
    const pass = enc.beginRenderPass({
      colorAttachments: [{
        view: gpu.ctx.getCurrentTexture().createView(),
        clearValue: { r: 0.043, g: 0.071, b: 0.125, a: 1 },
        loadOp: 'clear',
        storeOp: 'store'
      }]
    });
    pass.setPipeline(pipeline);
    pass.setVertexBuffer(0, vbo);
    pass.draw(verts.length / 2);
    pass.end();
    device.queue.submit([enc.finish()]);

    // Labels via 2D overlay would require a second canvas; skip — bars are the GPU path.
    console.log("[raptorBridge] WebGPU drew", idx.length, "logit bars");
  }
};

// ==============================================================================
// Interactive tour — language + web surfaces
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
    title: "2. Rich Control Flow & Decision Operators",
    desc: `
      <p>Raptor provides a rich suite of procedural, Raku-style, and short-circuit control flow constructs:</p>
      <ul>
        <li><strong>Conditionals:</strong> <code>if / elsif / else</code> and inverted <code>unless</code>.</li>
        <li><strong>Topical Pattern Matching:</strong> <code>given / when / default</code> with smartmatch (<code>~~</code>).</li>
        <li><strong>Loops:</strong> Pointy-block <code>for ... -&gt; $elem</code>, <code>while</code>, inverted <code>until</code>, and C-style <code>loop (;;)</code>.</li>
        <li><strong>Jump Operators:</strong> <code>last</code> (break), <code>next</code> (continue), and <code>return</code>.</li>
        <li><strong>Short-Circuit Operators:</strong> Raku ternary (<code>?? !!</code>) and defined-or defaulting (<code>//</code>, <code>//=</code>).</li>
      </ul>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# 1. Conditionals: if/elsif/else and unless
my $status = "active";
unless $status eq "disabled" {
    say "Service is operational.";
}

# 2. Topical Pattern Matching: given / when / default
my $score = 95;
given $score {
    when 90..100 { say "Score category: Distinction (A+)"; }
    when 80..89  { say "Score category: Merit (A)"; }
    default      { say "Score category: Pass ($_)"; }
}

# 3. Looping: for -> $x, until, and loop (;;)
say "--- Pointy-block For Loop with next / last ---";
for 1..6 -> $n {
    if $n == 2 { next; } # Skip 2
    if $n == 5 { last; } # Terminate before 5
    say "Processing item: ", $n;
}

say "--- Until Loop (Inverted Condition) ---";
my $ready_count = 0;
until $ready_count >= 3 {
    $ready_count += 1;
    say "Warming up... stage ", $ready_count;
}

# 4. Short-Circuit & Ternary Evaluation
my $val = Nil;
my $fallback = $val // "default_config";
my $label = ($ready_count == 3) ?? "Fully Ready" !! "Initializing";
say "Config: ", $fallback, " | State: ", $label;
`
  },

  {
    title: "3. Topic variable \$_ and Unicode math",
    desc: `
      <p>C<$_> is the default topic: C<for>, C<given>, and bare C<say> use it. Unicode operators C<× ÷ √> and names C<∑ ∏> are real identifiers.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `$_ = "hello topic";
say;   # prints $_

for 1..4 {
    say "n=", $_, "  n×n=", $_ × $_;
}

given 9 {
    when 9 { say "√", $_, " = ", √$_; }
}

say "sum ", ∑(1, 2, 3, 4, 5);
say "prod ", ∏(2, 3, 7);
`
  },

  {
    title: "4. Scoping: my, our, state",
    desc: `
      <p>Same three Perl 5 declarators:</p>
      <ul>
        <li><code>my</code> — lexical to the block</li>
        <li><code>our</code> — package-visible alias</li>
        <li><code>state</code> — persistent across calls</li>
      </ul>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `sub counter() {
    state $n = 0;
    $n = $n + 1;
    return $n;
}
say counter();
say counter();
say counter();

my $lex = "only here";
our $shared = "package";
say $lex, " / ", $shared;
`
  },

  {
    title: "5. Statement modifiers, labels, goto",
    desc: `
      <p>Postfix <code>if</code> / <code>unless</code> / <code>for</code>, plus labels and <code>goto</code> (including <code>goto &amp;sub</code>).</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `my $ok = True;
say "ok" if $ok;
say "hidden" unless $ok;

my $sum = 0;
$sum = $sum + $_ for 1..5;
say "sum ", $sum;

my $n = 0;
LOOP:
$n = $n + 1;
if $n < 3 { goto LOOP; }
say "n=", $n;
`
  },

  {
    title: "6. Contextual variables",
    desc: `
      <p>Raku-style dynamics: <code>@*ARGS</code>, <code>%*ENV</code>, <code>$*PID</code>, <code>$*RAPTOR</code>, <code>$*KERNEL</code>, <code>$?</code>, <code>$!</code>.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `say "name    ", $*RAPTOR{"name"};
say "version ", $*RAPTOR{"version"};
say "os      ", $*KERNEL{"name"};
say "arch    ", $*KERNEL{"arch"};
say "pid     ", $*PID;
say "status  ", $?;
say "env keys exist: ", %*ENV.elems() > 0;
`
  },

  {
    title: "7. References",
    desc: `
      <p>Take references with <code>\\</code> and dereference with <code>$$</code>, <code>@$</code>, arrows.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `my $val = 41;
my $sref = \\$val;
say "ref type ", ref($sref);
$$sref = 42;
say "via ref ", $val;

my @nums = [10, 20, 30];
my $aref = \\@nums;
say "first ", $aref->[0];
`
  },

  {
    title: "8. Uniform Function Call Syntax (UFCS)",
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
    title: "9. Lists, hashes, and backends",
    desc: `
      <p>Everyday list/hash ops. Backends (not switchable inside the browser): <code>--go</code> interpreter, <code>--moar</code> CompUnit v7, <code>raptor serve</code> for this WASM tour.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `my @xs = [3, 1, 2];
push(@xs, 4);
say "elems ", @xs.elems(), " sorted ", @xs.sort();

my %h = { "a" => 1, "b" => 2 };
say "keys ", keys(%h);
say "json ", to_json(%h);

say "this tour is the WASM backend (cmd/wasm)";
`
  },

  {
    title: "10. Dynamic Subsets & Continuous Invariants",
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
    title: "11. C-ABI Struct Records & Overloading",
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
    title: "12. Autothreading Quantum Junctions",
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
    title: "13. Signature Destructuring & Fast JSON",
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
    title: "14. Gather / Take Generators & Lazy Lists",
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
    title: "15. Packages and AUTOLOAD",
    desc: `
      <p>Namespaces, qualified calls, and fallback <code>AUTOLOAD</code> when a name is missing.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `package MathUtil;
sub add($a, $b) { return $a + $b; }
sub AUTOLOAD($x) {
    return "missing " ~ $AUTOLOAD ~ " arg=" ~ $x;
}

say MathUtil::add(2, 40);
say MathUtil::nope(7);
`
  },

  {
    title: "16. Grammars (gcre)",
    desc: `
      <p>Declarative <code>grammar</code> / <code>rule</code> / <code>token</code> objects. The language itself is parsed by <strong>gcre</strong> (a PEG-compatible Raku subset) with <code>&lt;HOST_stmt&gt;</code> calling the Pratt parser.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `grammar PointGrammar {
    token TOP { <num> ',' <num> }
    token num { \\d+ }
}
say "grammar name: ", PointGrammar{"name"};
say "TOP pattern stored: ", PointGrammar{"TOP"};
`
  },

  {
    title: "17. Design-by-Contract & Verification",
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
PROPERTY "Addition Commutativity" ($a, $b) {
    return ($a + $b) == ($b + $a);
}
`
  },

  {
    title: "18. TAP tests",
    desc: `
      <p>Test Anything Protocol builtins — the same ones <code>raptor test t/</code> runs. No <code>use Test::More</code> needed.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `plan 4;
ok 1 + 1 == 2, "one plus one";
is 2 ** 10, 1024, "pow";
is "ab" x 2, "abab", "repeat";
ok { True }, "done";
done_testing;
`
  },

  {
    title: "19. PodLit Literate Programming",
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
    title: "20. JSON, HTTP surface, and \$_ pipelines",
    disabled: true,
    desc: `
      <p>JSON is built in. Format HTTP responses. Pipe values through C<$_>.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `my %payload = { "lang" => "Raptor", "ok" => True };
my $js = to_json(%payload);
say $js;
my %back = from_json($js);
say %back{"lang"};

my $http = http_format_response(200, {:Server => "Raptor/1.0"}, $js);
say $http;

$_ = %back{"lang"};
say "topic still ", $_;
`
  },

  {
    title: "21. HTML5 Canvas 2D Graphics Engine",
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
    title: "22. WebGL 3D Hardware Graphics",
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
    title: "23. WebAudio Node Graph & Timeline",
    desc: `
      <p>This lesson builds a real <strong>Web Audio</strong> graph in Raptor: oscillators, ADSR gains, a low-pass filter, an LFO into <code>frequency</code>, a compressor, and a delay send.</p>
      <p>Notes are scheduled on <code>AudioContext.currentTime</code> — no <code>setTimeout</code>. The melody is the same C4–E4–G4–B4–C5 major-7th arpeggio.</p>
      <p>The JS bridge only exposes 1:1 AudioNode / AudioParam calls. Click <strong>Run</strong> (click once if the browser gated audio) to hear it.</p>
    `,
    defaultTab: "tabConsole",
    defaultView: "consoleView",
    code: `# WebAudio graph: osc -> ADSR -> lowpass(+LFO) -> compressor -> delay mix -> dest
# Melody: C4 E4 G4 B4 C5 on AudioContext.currentTime

my $ctx = audio_context_create();
my $t0 = audio_get_current_time($ctx);
say "sample rate ", audio_sample_rate($ctx), " Hz  t0=", $t0;

my $master = audio_create_gain($ctx);
audio_set_gain($master, 0.2, $t0);

my $filter = audio_create_biquad_filter($ctx, "lowpass");
audio_set_filter_freq($filter, 1800.0, $t0);
audio_set_filter_q($filter, 4.0, $t0);

my $lfo = audio_create_oscillator($ctx);
audio_set_osc_type($lfo, "sine");
audio_set_frequency($lfo, 4.5, $t0);
my $lfoGain = audio_create_gain($ctx);
audio_set_gain($lfoGain, 550.0, $t0);
audio_connect($lfo, $lfoGain);
audio_connect_param($lfoGain, $filter, "frequency");
audio_osc_start($lfo, $t0);

my $comp = audio_create_compressor($ctx);
audio_set_compressor($comp, -18.0, 12.0, 4.0, 0.003, 0.12);

my $delay = audio_create_delay($ctx, 0.28);
audio_set_delay_time($delay, 0.2, $t0);
my $fb = audio_create_gain($ctx);
audio_set_gain($fb, 0.22, $t0);

my $pan = audio_create_panner($ctx);
audio_set_pan($pan, -0.15, $t0);

audio_connect($filter, $master);
audio_connect($master, $comp);
audio_connect($comp, $pan);
audio_connect($pan, $delay);
audio_connect($delay, $fb);
audio_connect($fb, $comp);
audio_connect_destination($pan, $ctx);

my $an = audio_create_analyser($ctx);
audio_set_fft_size($an, 256);
audio_connect($master, $an);

my @melody = [261.63, 329.63, 392.00, 493.88, 523.25];
my @durations = [0.18, 0.18, 0.18, 0.18, 0.42];
my @waves = ["triangle", "triangle", "sine", "triangle", "sine"];

my $t = $t0 + 0.05;
my $i = 0;
while $i < 5 {
    my $osc = audio_create_oscillator($ctx);
    my $env = audio_create_gain($ctx);
    audio_set_osc_type($osc, @waves[$i]);
    audio_set_frequency($osc, @melody[$i], $t);
    audio_set_detune($osc, ($i - 2) * 3.0, $t);
    audio_set_gain($env, 0.0001, $t);
    audio_gain_ramp_exp($env, 0.85, $t + 0.02);
    audio_gain_ramp_exp($env, 0.0001, $t + @durations[$i]);
    audio_connect($osc, $env);
    audio_connect($env, $filter);
    audio_osc_start($osc, $t);
    audio_osc_stop($osc, $t + @durations[$i] + 0.03);
    $t = $t + @durations[$i];
    $i = $i + 1;
}

say "graph: osc+ADSR -> LFO-mod lowpass -> compressor -> panner+delay";
say "scheduled C4 E4 G4 B4 C5 on the AudioContext timeline";
`
  },

  {
    title: "24. WebGPU tiny LLM",
    disabled: true,
    desc: `
      <p>Raptor loads a <strong>tiny character-level language model</strong> (n-gram mix + a linear layer) and runs next-token prediction from the script.</p>
      <p><code>webgpu_init</code> requests a GPU adapter. Linear-layer matmuls kick a WGSL compute pipeline when WebGPU is available; generation still returns immediately. Last-step logits are drawn as bars on the WebGPU viewport.</p>
      <p>The same <code>llm_tiny_*</code> builtins run on desktop via the CPU / GGML-shaped tensor path.</p>
    `,
    defaultTab: "tabWebGPU",
    defaultView: "webgpuView",
    code: `# Tiny char LM via Raptor. WebGPU compute for the linear layer
# when the adapter is ready; otherwise the same tensors on CPU.

webgpu_init("wasmWebGPUCanvas", 640, 320);
say "webgpu available: ", webgpu_available();

my $model = llm_tiny_load();
say "tiny LLM backend: ", llm_tiny_backend();

my $prompt = "raptor is ";
my $out = llm_tiny_generate($model, $prompt, 48, 0.35);
say $out;

my @logits = llm_tiny_logits($model, $out);
webgpu_draw_logits(@logits);
say "drew ", @logits.elems(), " logits  vocab=", llm_tiny_vocab();
`
  }
];

// Disabled lessons stay in source but are skipped in the stepper/dropdown.
for (let i = tourLessons.length - 1; i >= 0; i--) {
  if (tourLessons[i].disabled) tourLessons.splice(i, 1);
}

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
function rebuildLessonSelect() {
  if (!lessonSelect) return;
  lessonSelect.innerHTML = tourLessons.map((l, i) => {
    const label = String(l.title || '').replace(/&/g, '&amp;');
    return `<option value="${i}">${label}</option>`;
  }).join('');
}

function initTour() {
  rebuildLessonSelect();
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
        codeEditor.value = lesson.code.trim() + '\n';
        updateLineNumbers();
      }
      if (consoleTerminal) {
        consoleTerminal.innerHTML = '';
        appendToConsole("Reset: Editor code, canvas, and console restored to initial lesson state.", "output");
      }
      const cv2d = document.getElementById('wasmCanvas');
      if (cv2d) {
        const c2dCtx = cv2d.getContext('2d');
        if (c2dCtx) c2dCtx.clearRect(0, 0, cv2d.width, cv2d.height);
      }
      if (webglAnimId) {
        cancelAnimationFrame(webglAnimId);
        webglAnimId = null;
        isWebglAnimating = false;
      }
      if (codeEditor) {
        codeEditor.style.transition = 'background-color 0.2s';
        codeEditor.style.backgroundColor = '#e8f4fd';
        setTimeout(() => {
          codeEditor.style.backgroundColor = '';
        }, 250);
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
  const tg = document.getElementById('tabWebGPU');
  const td = document.getElementById('tabDom');
  if (tc) tc.addEventListener('click', () => switchToTab('tabConsole', 'consoleView'));
  if (tcv) tcv.addEventListener('click', () => switchToTab('tabCanvas', 'canvasView'));
  if (tw) tw.addEventListener('click', () => switchToTab('tabWebGL', 'webglView'));
  if (tg) tg.addEventListener('click', () => switchToTab('tabWebGPU', 'webgpuView'));
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

// Initialize WebAssembly Runtime with robust fallback
async function initWasm() {
  const go = new Go();
  statusDot.className = 'status-dot';
  statusText.textContent = 'Loading WebAssembly...';

  try {
    const wasmUrl = 'raptor.wasm';
    const response = await fetch(wasmUrl);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status} (${response.statusText || 'Not Found'}) loading ${wasmUrl}`);
    }

    const source = await response.arrayBuffer();
    const result = await WebAssembly.instantiate(source, go.importObject);
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
  } catch (err) {
    statusDot.className = 'status-dot error';
    statusText.textContent = 'Wasm Load Error';
    appendToConsole("Failed loading raptor.wasm: " + err.message, "error");
    console.error("[WASM Loader Error]", err);
  }
}

// Start Tour when DOM is ready
document.addEventListener('DOMContentLoaded', initTour);

