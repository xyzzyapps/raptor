// Raptor WASM stubs — embedded next to raptor.wasm so any page can
// load canvas / WebAudio / WebGL / WebGPU without the tour HTML.
(function (global) {
  if (global.raptorBridge) return;

  let audioCtx = null;
  let audioNodeSeq = 1;
  const audioNodes = {};
  let canvasContexts = [];
  let gl = null;
  let glShaders = [], glPrograms = [], glBuffers = [], glUniformLocs = [];

  function switchTab(tab, view) {
    if (typeof global.switchToTab === "function") {
      global.switchToTab(tab, view);
    }
  }

  const bridge = {
    canvasGetContext: function (canvasId, width, height) {
      const canvas = document.getElementById(canvasId) || document.getElementById("wasmCanvas");
      if (!canvas) return 0;
      if (width) canvas.width = width;
      if (height) canvas.height = height;
      canvasContexts = [canvas.getContext("2d")];
      return 0;
    },
    canvasSetFillStyle: function (id, c) { if (canvasContexts[id]) canvasContexts[id].fillStyle = c; },
    canvasSetStrokeStyle: function (id, c) { if (canvasContexts[id]) canvasContexts[id].strokeStyle = c; },
    canvasSetLineWidth: function (id, w) { if (canvasContexts[id]) canvasContexts[id].lineWidth = w; },
    canvasSetFont: function (id, f) { if (canvasContexts[id]) canvasContexts[id].font = f; },
    canvasFillRect: function (id, x, y, w, h) { if (canvasContexts[id]) canvasContexts[id].fillRect(x, y, w, h); },
    canvasStrokeRect: function (id, x, y, w, h) { if (canvasContexts[id]) canvasContexts[id].strokeRect(x, y, w, h); },
    canvasClearRect: function (id, x, y, w, h) { if (canvasContexts[id]) canvasContexts[id].clearRect(x, y, w, h); },
    canvasBeginPath: function (id) { if (canvasContexts[id]) canvasContexts[id].beginPath(); },
    canvasClosePath: function (id) { if (canvasContexts[id]) canvasContexts[id].closePath(); },
    canvasMoveTo: function (id, x, y) { if (canvasContexts[id]) canvasContexts[id].moveTo(x, y); },
    canvasLineTo: function (id, x, y) { if (canvasContexts[id]) canvasContexts[id].lineTo(x, y); },
    canvasArc: function (id, x, y, r, a, b) { if (canvasContexts[id]) canvasContexts[id].arc(x, y, r, a, b); },
    canvasStroke: function (id) { if (canvasContexts[id]) canvasContexts[id].stroke(); },
    canvasFill: function (id) { if (canvasContexts[id]) canvasContexts[id].fill(); },
    canvasFillText: function (id, t, x, y) { if (canvasContexts[id]) canvasContexts[id].fillText(t, x, y); },

    _audioAlloc: function (kind, node, ctx, extra) {
      const id = audioNodeSeq++;
      audioNodes[id] = { kind, node, ctx, extra: extra || null };
      return id;
    },
    _audioNode: function (id) { return audioNodes[id] ? audioNodes[id].node : null; },
    _audioCtx: function (id) {
      const e = audioNodes[id];
      if (e && e.kind === "context") return e.node;
      if (e && e.ctx) return e.ctx;
      return audioCtx;
    },
    _audioResume: function (ctx) {
      if (ctx && ctx.state === "suspended") ctx.resume().catch(function () {});
    },
    initAudio: function () {
      if (!audioCtx) {
        const AC = global.AudioContext || global.webkitAudioContext;
        audioCtx = new AC();
      }
      this._audioResume(audioCtx);
    },
    audioContextCreate: function () {
      const AC = global.AudioContext || global.webkitAudioContext;
      const ctx = new AC();
      this._audioResume(ctx);
      audioCtx = ctx;
      const destId = this._audioAlloc("destination", ctx.destination, ctx);
      return this._audioAlloc("context", ctx, ctx, { destId });
    },
    audioGetCurrentTime: function (id) { const c = this._audioCtx(id); return c ? c.currentTime : 0; },
    audioSampleRate: function (id) { const c = this._audioCtx(id); return c ? c.sampleRate : 44100; },
    audioDestination: function (id) {
      const e = audioNodes[id];
      if (e && e.extra && e.extra.destId) return e.extra.destId;
      const ctx = this._audioCtx(id);
      return ctx ? this._audioAlloc("destination", ctx.destination, ctx) : 0;
    },
    audioCreateOscillator: function (id) { const c = this._audioCtx(id); return c ? this._audioAlloc("oscillator", c.createOscillator(), c) : 0; },
    audioCreateGain: function (id) { const c = this._audioCtx(id); return c ? this._audioAlloc("gain", c.createGain(), c) : 0; },
    audioCreateBiquadFilter: function (id, typ) {
      const c = this._audioCtx(id); if (!c) return 0;
      const f = c.createBiquadFilter(); f.type = typ || "lowpass";
      return this._audioAlloc("filter", f, c);
    },
    audioCreateCompressor: function (id) { const c = this._audioCtx(id); return c ? this._audioAlloc("compressor", c.createDynamicsCompressor(), c) : 0; },
    audioCreateDelay: function (id, max) { const c = this._audioCtx(id); return c ? this._audioAlloc("delay", c.createDelay(max || 1), c) : 0; },
    audioCreatePanner: function (id) { const c = this._audioCtx(id); return c ? this._audioAlloc("panner", c.createStereoPanner(), c) : 0; },
    audioCreateAnalyser: function (id) { const c = this._audioCtx(id); return c ? this._audioAlloc("analyser", c.createAnalyser(), c) : 0; },
    audioCreateBuffer: function (id, ch, len, sr) {
      const c = this._audioCtx(id); if (!c) return 0;
      return this._audioAlloc("buffer", c.createBuffer(ch || 1, len || c.sampleRate, sr || c.sampleRate), c);
    },
    audioCreateBufferSource: function (id) { const c = this._audioCtx(id); return c ? this._audioAlloc("source", c.createBufferSource(), c) : 0; },
    audioConnect: function (s, d) { const a = this._audioNode(s), b = this._audioNode(d); if (a && b && a.connect) try { a.connect(b); } catch (e) {} },
    audioConnectParam: function (s, d, p) { const a = this._audioNode(s), b = this._audioNode(d); if (a && b && b[p]) try { a.connect(b[p]); } catch (e) {} },
    audioConnectDestination: function (s, id) { const a = this._audioNode(s), c = this._audioCtx(id); if (a && c) try { a.connect(c.destination); } catch (e) {} },
    audioDisconnect: function (s) { const a = this._audioNode(s); if (a && a.disconnect) try { a.disconnect(); } catch (e) {} },
    audioSetOscType: function (id, t) { const n = this._audioNode(id); if (n) n.type = t || "sine"; },
    audioSetFrequency: function (id, f, t) { const n = this._audioNode(id); if (n && n.frequency) n.frequency.setValueAtTime(f, t || 0); },
    audioFreqRamp: function (id, f, t) { const n = this._audioNode(id); if (n && n.frequency) n.frequency.exponentialRampToValueAtTime(Math.max(f, 1), t); },
    audioSetDetune: function (id, c, t) { const n = this._audioNode(id); if (n && n.detune) n.detune.setValueAtTime(c, t || 0); },
    audioSetGain: function (id, v, t) { const n = this._audioNode(id); if (n && n.gain) n.gain.setValueAtTime(v, t || 0); },
    audioGainRampExp: function (id, v, t) { const n = this._audioNode(id); if (n && n.gain) n.gain.exponentialRampToValueAtTime(Math.max(v, 1e-5), t); },
    audioGainRampLinear: function (id, v, t) { const n = this._audioNode(id); if (n && n.gain) n.gain.linearRampToValueAtTime(v, t); },
    audioSetFilterFreq: function (id, f, t) { const n = this._audioNode(id); if (n && n.frequency) n.frequency.setValueAtTime(f, t || 0); },
    audioSetFilterQ: function (id, q, t) { const n = this._audioNode(id); if (n && n.Q) n.Q.setValueAtTime(q, t || 0); },
    audioSetCompressor: function (id, thr, knee, ratio, atk, rel) {
      const c = this._audioNode(id); if (!c || !c.threshold) return;
      const t = c.context ? c.context.currentTime : 0;
      c.threshold.setValueAtTime(thr, t); c.knee.setValueAtTime(knee, t);
      c.ratio.setValueAtTime(ratio, t); c.attack.setValueAtTime(atk, t); c.release.setValueAtTime(rel, t);
    },
    audioSetDelayTime: function (id, s, t) { const n = this._audioNode(id); if (n && n.delayTime) n.delayTime.setValueAtTime(s, t || 0); },
    audioSetPan: function (id, p, t) { const n = this._audioNode(id); if (n && n.pan) n.pan.setValueAtTime(p, t || 0); },
    audioSetFftSize: function (id, n) { const a = this._audioNode(id); if (a) a.fftSize = n || 256; },
    audioGetSpectrum: function (id) {
      const a = this._audioNode(id); if (!a || !a.getByteFrequencyData) return [];
      const buf = new Uint8Array(a.frequencyBinCount); a.getByteFrequencyData(buf); return Array.from(buf);
    },
    audioBufferFillSine: function (id, freq) {
      const buf = this._audioNode(id); if (!buf || !buf.getChannelData) return;
      const data = buf.getChannelData(0), sr = buf.sampleRate, f = freq || 440;
      for (let i = 0; i < data.length; i++) data[i] = Math.sin(2 * Math.PI * f * i / sr) * 0.3;
    },
    audioSourceSetBuffer: function (s, b) { const src = this._audioNode(s), buf = this._audioNode(b); if (src && buf) src.buffer = buf; },
    audioSourceStart: function (id, t) { const n = this._audioNode(id); if (n && n.start) try { n.start(t || 0); } catch (e) {} },
    audioOscStart: function (id, t) { const n = this._audioNode(id); if (n && n.start) try { n.start(t || 0); } catch (e) {} },
    audioOscStop: function (id, t) { const n = this._audioNode(id); if (n && n.stop) try { n.stop(t || 0); } catch (e) {} },

    glInit: function (canvasId, w, h) {
      const canvas = document.getElementById(canvasId) || document.getElementById("wasmWebGLCanvas");
      if (!canvas) return;
      if (w) canvas.width = w; if (h) canvas.height = h;
      gl = canvas.getContext("webgl") || canvas.getContext("experimental-webgl");
      glShaders = []; glPrograms = []; glBuffers = []; glUniformLocs = [];
      if (gl) gl.viewport(0, 0, canvas.width, canvas.height);
    },
    glClearColor: function (r, g, b, a) { if (gl) gl.clearColor(r, g, b, a); },
    glClear: function () { if (gl) gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT); },
    glEnableDepthTest: function () { if (gl) gl.enable(gl.DEPTH_TEST); },
    glCreateShader: function (typ) {
      if (!gl) return 0;
      const s = gl.createShader(typ === "FRAGMENT" ? gl.FRAGMENT_SHADER : gl.VERTEX_SHADER);
      glShaders.push(s); return glShaders.length - 1;
    },
    glShaderSource: function (id, src) { if (gl && glShaders[id]) gl.shaderSource(glShaders[id], src); },
    glCompileShader: function (id) { if (gl && glShaders[id]) gl.compileShader(glShaders[id]); },
    glCreateProgram: function () { if (!gl) return 0; glPrograms.push(gl.createProgram()); return glPrograms.length - 1; },
    glAttachShader: function (p, s) { if (gl && glPrograms[p] && glShaders[s]) gl.attachShader(glPrograms[p], glShaders[s]); },
    glLinkProgram: function (p) { if (gl && glPrograms[p]) gl.linkProgram(glPrograms[p]); },
    glUseProgram: function (p) { if (gl && glPrograms[p]) gl.useProgram(glPrograms[p]); },
    glGetAttribLocation: function (p, n) { return (gl && glPrograms[p]) ? gl.getAttribLocation(glPrograms[p], n) : -1; },
    glGetUniformLocation: function (p, n) {
      if (!gl || !glPrograms[p]) return -1;
      glUniformLocs.push(gl.getUniformLocation(glPrograms[p], n));
      return glUniformLocs.length - 1;
    },
    glEnableVertexAttribArray: function (loc) { if (gl && loc >= 0) gl.enableVertexAttribArray(loc); },
    glCreateBuffer: function () { if (!gl) return 0; glBuffers.push(gl.createBuffer()); return glBuffers.length - 1; },
    glBindBuffer: function (t, id) { if (gl && glBuffers[id]) gl.bindBuffer(t === "ELEMENT" ? gl.ELEMENT_ARRAY_BUFFER : gl.ARRAY_BUFFER, glBuffers[id]); },
    glBufferData: function (t, data) {
      if (!gl || !data) return;
      const arr = Array.isArray(data) ? data : Array.from(data);
      const tgt = t === "ELEMENT" ? gl.ELEMENT_ARRAY_BUFFER : gl.ARRAY_BUFFER;
      gl.bufferData(tgt, t === "ELEMENT" ? new Uint16Array(arr) : new Float32Array(arr), gl.STATIC_DRAW);
    },
    glVertexAttribPointer: function (loc, size) { if (gl && loc >= 0) gl.vertexAttribPointer(loc, size, gl.FLOAT, false, 0, 0); },
    glUniformMatrix4fv: function (id, m) {
      if (gl && glUniformLocs[id] && m) gl.uniformMatrix4fv(glUniformLocs[id], false, new Float32Array(Array.isArray(m) ? m : Array.from(m)));
    },
    glDrawElements: function (n) { if (gl) gl.drawElements(gl.TRIANGLES, n || 36, gl.UNSIGNED_SHORT, 0); },
    glStartAnimation: function () {},

    webgpuReady: false,
    _gpu: null,
    webgpuInit: function (canvasId, width, height, silent) {
      const self = this;
      const canvas = document.getElementById(canvasId) || document.getElementById("wasmWebGPUCanvas");
      if (canvas) {
        if (width) canvas.width = width;
        if (height) canvas.height = height;
        if (!silent) switchTab("tabWebGPU", "webgpuView");
      }
      if (!navigator.gpu) { self.webgpuReady = false; return 0; }
      if (self._gpu && self._gpu.pending) return 1;
      self._gpu = { pending: true };
      (async function () {
        try {
          const adapter = await navigator.gpu.requestAdapter();
          if (!adapter) { self.webgpuReady = false; self._gpu.pending = false; return; }
          const device = await adapter.requestDevice();
          const format = navigator.gpu.getPreferredCanvasFormat();
          let ctx = null;
          if (canvas) {
            ctx = canvas.getContext("webgpu");
            if (ctx) ctx.configure({ device: device, format: format, alphaMode: "opaque" });
          }
          const shader = device.createShaderModule({
            code: `
              struct Dims { m: u32, n: u32, k: u32, _pad: u32 }
              @group(0) @binding(0) var<storage, read> A: array<f32>;
              @group(0) @binding(1) var<storage, read> B: array<f32>;
              @group(0) @binding(2) var<storage, read_write> C: array<f32>;
              @group(0) @binding(3) var<uniform> dims: Dims;
              @compute @workgroup_size(8, 8)
              fn main(@builtin(global_invocation_id) gid: vec3u) {
                let row = gid.x; let col = gid.y;
                if (row >= dims.m || col >= dims.n) { return; }
                var acc = 0.0;
                for (var t = 0u; t < dims.k; t++) {
                  acc += A[row * dims.k + t] * B[t * dims.n + col];
                }
                C[row * dims.n + col] = acc;
              }`
          });
          const pipeline = device.createComputePipeline({ layout: "auto", compute: { module: shader, entryPoint: "main" } });
          self._gpu = { device: device, canvas: canvas, ctx: ctx, format: format, pipeline: pipeline, pending: false };
          self.webgpuReady = true;
        } catch (err) {
          console.warn("[raptorBridge] WebGPU init failed", err);
          self.webgpuReady = false;
          if (self._gpu) self._gpu.pending = false;
        }
      })();
      return 1;
    },
    webgpuAvailable: function () { return !!(this.webgpuReady && this._gpu && this._gpu.device); },
    _cpuMatmul: function (m, n, k, A, B) {
      const C = new Array(m * n).fill(0);
      for (let i = 0; i < m; i++) for (let j = 0; j < n; j++) {
        let acc = 0; for (let t = 0; t < k; t++) acc += A[i * k + t] * B[t * n + j];
        C[i * n + j] = acc;
      }
      return C;
    },
    webgpuMatmul: function (m, n, k, a, b) {
      const A = Array.isArray(a) ? a : Array.from(a || []);
      const B = Array.isArray(b) ? b : Array.from(b || []);
      return this._cpuMatmul(m, n, k, A, B);
    },
    webgpuMatmulAsync: function (m, n, k, a, b, cb) {
      const self = this;
      const A = Array.isArray(a) ? a : Array.from(a || []);
      const B = Array.isArray(b) ? b : Array.from(b || []);
      const fallback = function () { cb(self._cpuMatmul(m, n, k, A, B)); };
      if (!self.webgpuAvailable()) { fallback(); return; }
      self._webgpuMatmulRead(m, n, k, A, B).then(function (C) { cb(C); }).catch(fallback);
    },
    _webgpuMatmulRead: async function (m, n, k, A, B) {
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
    webgpuDrawLogits: function (logits, vocab) {
      const arr = Array.isArray(logits) ? logits : Array.from(logits || []);
      const canvas = (this._gpu && this._gpu.canvas) || document.getElementById("wasmWebGPUCanvas");
      if (!canvas) return;
      let maxv = -Infinity;
      for (let i = 0; i < arr.length; i++) if (arr[i] > maxv) maxv = arr[i];
      const probs = arr.map(function (v) { return Math.exp(v - maxv); });
      let sum = 0; for (let i = 0; i < probs.length; i++) sum += probs[i];
      const norm = probs.map(function (p) { return sum ? p / sum : 0; });
      const idx = norm.map(function (_, i) { return i; }).sort(function (a, b) { return norm[b] - norm[a]; }).slice(0, 16);
      if (this.webgpuAvailable() && this._gpu.ctx) return;
      if (this._gpu && this._gpu.ctx) return;
      const ctx2d = canvas.getContext("2d");
      if (!ctx2d) return;
      const w = canvas.width, h = canvas.height;
      ctx2d.fillStyle = "#0b1220"; ctx2d.fillRect(0, 0, w, h);
      const barW = Math.max(8, (w - 40) / idx.length - 6);
      idx.forEach(function (vi, i) {
        const bh = norm[vi] * (h - 70);
        ctx2d.fillStyle = "#007d9c";
        ctx2d.fillRect(20 + i * (barW + 6), h - 28 - bh, barW, bh);
      });
    }
  };

  global.raptorBridge = bridge;
})(typeof window !== "undefined" ? window : globalThis);
