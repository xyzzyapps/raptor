//go:build js && wasm

package raptor

import (
	"fmt"
	"syscall/js"
)

func wasmLog(prefix string, msg string) {
	console := js.Global().Get("console")
	if !console.IsNull() && !console.IsUndefined() {
		console.Call("log", fmt.Sprintf("[Raptor WASM] [%s] %s", prefix, msg))
	}
}

func wasmErr(prefix string, msg string) {
	console := js.Global().Get("console")
	if !console.IsNull() && !console.IsUndefined() {
		console.Call("error", fmt.Sprintf("[Raptor WASM Error] [%s] %s", prefix, msg))
	}
}

func (in *Interp) registerFFI() {
	in.Builtins["ffi_load"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("native FFI is not supported in WebAssembly environment")
	}
	in.Builtins["ffi_call"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("native FFI is not supported in WebAssembly environment")
	}
}

func (in *Interp) registerPortAudioBuiltins() {
	in.Builtins["pa_init"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("PortAudio is not supported in WebAssembly environment (use WebAudio builtins)")
	}
	in.Builtins["pa_terminate"] = func(in *Interp, args []*Value) (*Value, error) {
		return NilValue(), nil
	}
}

func (in *Interp) registerSQLiteBuiltins() {
	in.Builtins["sqlite_open"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("native SQLite is not supported in WebAssembly environment")
	}
}

func (in *Interp) registerWebBuiltins() {
	// DOM Manipulation
	in.Builtins["dom_get"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		selector := args[0].String()
		doc := js.Global().Get("document")
		elem := doc.Call("querySelector", selector)
		if elem.IsNull() || elem.IsUndefined() {
			return NilValue(), nil
		}
		return StringValue(elem.Get("textContent").String()), nil
	}

	in.Builtins["dom_set_text"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return NilValue(), nil
		}
		selector := args[0].String()
		text := args[1].String()
		doc := js.Global().Get("document")
		elem := doc.Call("querySelector", selector)
		if !elem.IsNull() && !elem.IsUndefined() {
			elem.Set("textContent", text)
			return BoolValue(true), nil
		}
		return BoolValue(false), nil
	}

	in.Builtins["dom_set_html"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return NilValue(), nil
		}
		selector := args[0].String()
		html := args[1].String()
		doc := js.Global().Get("document")
		elem := doc.Call("querySelector", selector)
		if !elem.IsNull() && !elem.IsUndefined() {
			elem.Set("innerHTML", html)
			return BoolValue(true), nil
		}
		return BoolValue(false), nil
	}

	in.Builtins["dom_create"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return NilValue(), nil
		}
		parentSel := args[0].String()
		tag := args[1].String()
		text := ""
		if len(args) > 2 {
			text = args[2].String()
		}
		className := ""
		if len(args) > 3 {
			className = args[3].String()
		}

		doc := js.Global().Get("document")
		parent := doc.Call("querySelector", parentSel)
		if parent.IsNull() || parent.IsUndefined() {
			return BoolValue(false), nil
		}

		elem := doc.Call("createElement", tag)
		if text != "" {
			elem.Set("textContent", text)
		}
		if className != "" {
			elem.Set("className", className)
		}
		parent.Call("appendChild", elem)
		return BoolValue(true), nil
	}

	// HTML5 Canvas 2D
	in.Builtins["canvas_init"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			id := "wasmCanvas"
			w := 640
			h := 400
			if len(args) > 0 {
				id = args[0].String()
			}
			if len(args) > 2 {
				w = int(args[1].IntVal)
				h = int(args[2].IntVal)
			}
			bridge.Call("initCanvas", id, w, h)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_clear"] = func(in *Interp, args []*Value) (*Value, error) {
		color := "#0f172a"
		if len(args) > 0 {
			color = args[0].String()
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("clearCanvas", color)
		}
		return BoolValue(true), nil
	}

	// --- Low-Level HTML5 Canvas 2D Built-ins ---
	in.Builtins["canvas_get_context"] = func(in *Interp, args []*Value) (*Value, error) {
		id := "wasmCanvas"
		w := 640
		h := 380
		if len(args) > 0 {
			id = args[0].String()
		}
		if len(args) > 2 {
			w = int(in.toInt(args[1]))
			h = int(in.toInt(args[2]))
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			ctxId := bridge.Call("canvasGetContext", id, w, h).Int()
			wasmLog("Canvas2D", fmt.Sprintf("canvas_get_context(id=%s, w=%d, h=%d) -> ctxId=%d", id, w, h, ctxId))
			return IntValue(int64(ctxId)), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["canvas_set_fill_style"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		color := args[1].String()
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasSetFillStyle", ctxId, color)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_set_stroke_style"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		color := args[1].String()
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasSetStrokeStyle", ctxId, color)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_set_line_width"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		lw := toFloat(args[1])
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasSetLineWidth", ctxId, lw)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_set_font"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		font := args[1].String()
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasSetFont", ctxId, font)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_fill_rect"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 5 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		x := toFloat(args[1])
		y := toFloat(args[2])
		w := toFloat(args[3])
		h := toFloat(args[4])
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasFillRect", ctxId, x, y, w, h)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_stroke_rect"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 5 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		x := toFloat(args[1])
		y := toFloat(args[2])
		w := toFloat(args[3])
		h := toFloat(args[4])
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasStrokeRect", ctxId, x, y, w, h)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_clear_rect"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 5 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		x := toFloat(args[1])
		y := toFloat(args[2])
		w := toFloat(args[3])
		h := toFloat(args[4])
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasClearRect", ctxId, x, y, w, h)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_begin_path"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasBeginPath", ctxId)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_close_path"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasClosePath", ctxId)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_move_to"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		x := toFloat(args[1])
		y := toFloat(args[2])
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasMoveTo", ctxId, x, y)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_line_to"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		x := toFloat(args[1])
		y := toFloat(args[2])
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasLineTo", ctxId, x, y)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_arc"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 6 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		x := toFloat(args[1])
		y := toFloat(args[2])
		r := toFloat(args[3])
		sAngle := toFloat(args[4])
		eAngle := toFloat(args[5])
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasArc", ctxId, x, y, r, sAngle, eAngle)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_stroke"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasStroke", ctxId)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_fill"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasFill", ctxId)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_fill_text"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 4 {
			return BoolValue(false), nil
		}
		ctxId := int(in.toInt(args[0]))
		text := args[1].String()
		x := toFloat(args[2])
		y := toFloat(args[3])
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("canvasFillText", ctxId, text, x, y)
		}
		return BoolValue(true), nil
	}

	// WebAudio API
	// --- Low-Level WebAudio DSP Node Built-ins ---
	in.Builtins["audio_context_create"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			ctxId := bridge.Call("audioContextCreate").Int()
			wasmLog("WebAudio", fmt.Sprintf("audio_context_create() -> ctxId=%d", ctxId))
			return IntValue(int64(ctxId)), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["audio_get_current_time"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			t := bridge.Call("audioGetCurrentTime", ctxId).Float()
			return FloatValue(t), nil
		}
		return FloatValue(0.0), nil
	}

	in.Builtins["audio_create_oscillator"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			oscId := bridge.Call("audioCreateOscillator", ctxId).Int()
			wasmLog("WebAudio", fmt.Sprintf("audio_create_oscillator(ctx=%d) -> oscId=%d", ctxId, oscId))
			return IntValue(int64(oscId)), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["audio_create_gain"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			gainId := bridge.Call("audioCreateGain", ctxId).Int()
			wasmLog("WebAudio", fmt.Sprintf("audio_create_gain(ctx=%d) -> gainId=%d", ctxId, gainId))
			return IntValue(int64(gainId)), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["audio_create_biquad_filter"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		filterType := "lowpass"
		if len(args) > 1 {
			filterType = args[1].String()
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			filterId := bridge.Call("audioCreateBiquadFilter", ctxId, filterType).Int()
			wasmLog("WebAudio", fmt.Sprintf("audio_create_biquad_filter(ctx=%d, type=%s) -> filterId=%d", ctxId, filterType, filterId))
			return IntValue(int64(filterId)), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["audio_connect"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		srcId := int(in.toInt(args[0]))
		dstId := int(in.toInt(args[1]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioConnect", srcId, dstId)
			wasmLog("WebAudio", fmt.Sprintf("audio_connect(src=%d, dst=%d)", srcId, dstId))
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_connect_destination"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		srcId := int(in.toInt(args[0]))
		ctxId := 0
		if len(args) > 1 {
			ctxId = int(in.toInt(args[1]))
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioConnectDestination", srcId, ctxId)
			wasmLog("WebAudio", fmt.Sprintf("audio_connect_destination(src=%d, ctx=%d)", srcId, ctxId))
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_set_osc_type"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		oscId := int(in.toInt(args[0]))
		waveType := args[1].String()
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioSetOscType", oscId, waveType)
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_set_frequency"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		oscId := int(in.toInt(args[0]))
		freq := toFloat(args[1])
		timeOffset := 0.0
		if len(args) > 2 {
			timeOffset = toFloat(args[2])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioSetFrequency", oscId, freq, timeOffset)
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_set_gain"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		gainId := int(in.toInt(args[0]))
		gainVal := toFloat(args[1])
		timeOffset := 0.0
		if len(args) > 2 {
			timeOffset = toFloat(args[2])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioSetGain", gainId, gainVal, timeOffset)
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_gain_ramp_exp"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		gainId := int(in.toInt(args[0]))
		targetVal := toFloat(args[1])
		endTime := 0.0
		if len(args) > 2 {
			endTime = toFloat(args[2])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioGainRampExp", gainId, targetVal, endTime)
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_gain_ramp_linear"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		gainId := int(in.toInt(args[0]))
		targetVal := toFloat(args[1])
		endTime := 0.0
		if len(args) > 2 {
			endTime = toFloat(args[2])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioGainRampLinear", gainId, targetVal, endTime)
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_set_filter_freq"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		filterId := int(in.toInt(args[0]))
		freq := toFloat(args[1])
		timeOffset := 0.0
		if len(args) > 2 {
			timeOffset = toFloat(args[2])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioSetFilterFreq", filterId, freq, timeOffset)
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_osc_start"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		oscId := int(in.toInt(args[0]))
		startTime := 0.0
		if len(args) > 1 {
			startTime = toFloat(args[1])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioOscStart", oscId, startTime)
			wasmLog("WebAudio", fmt.Sprintf("audio_osc_start(osc=%d, startTime=%.3f)", oscId, startTime))
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_osc_stop"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		oscId := int(in.toInt(args[0]))
		stopTime := 0.0
		if len(args) > 1 {
			stopTime = toFloat(args[1])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("audioOscStop", oscId, stopTime)
			wasmLog("WebAudio", fmt.Sprintf("audio_osc_stop(osc=%d, stopTime=%.3f)", oscId, stopTime))
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_init"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("initAudio")
			wasmLog("WebAudio", "audio_init() ready")
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_play_tone"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		freq := toFloat(args[0])
		dur := 0.2
		if len(args) > 1 {
			dur = toFloat(args[1])
		}
		waveform := "triangle"
		if len(args) > 2 {
			waveform = args[2].String()
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("playTone", freq, dur, waveform)
			wasmLog("WebAudio", fmt.Sprintf("audio_play_tone(freq=%.2f, dur=%.2f, wave=%s)", freq, dur, waveform))
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_play_melody"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		jsFreqs := js.Global().Get("Array").New()
		if args[0].Type == ValArray {
			for i, item := range args[0].ArrayVal {
				jsFreqs.SetIndex(i, toFloat(item))
			}
		}
		jsDurs := js.Global().Get("Array").New()
		if len(args) > 1 && args[1].Type == ValArray {
			for i, item := range args[1].ArrayVal {
				jsDurs.SetIndex(i, toFloat(item))
			}
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("playMelody", jsFreqs, jsDurs)
			wasmLog("WebAudio", fmt.Sprintf("audio_play_melody(notes=%d)", jsFreqs.Length()))
		}
		return BoolValue(true), nil
	}

	// WebGL 3D Low-Level GPU Hardware Accelerated Bindings
	in.Builtins["gl_init"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			id := "wasmWebGLCanvas"
			w := 640
			h := 380
			if len(args) > 0 {
				id = args[0].String()
			}
			if len(args) > 2 {
				w = int(in.toInt(args[1]))
				h = int(in.toInt(args[2]))
			}
			bridge.Call("glInit", id, w, h)
			wasmLog("WebGL", fmt.Sprintf("gl_init(id=%s, w=%d, h=%d)", id, w, h))
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_clear_color"] = func(in *Interp, args []*Value) (*Value, error) {
		r, g, b, a := 0.97, 0.98, 0.99, 1.0
		if len(args) >= 3 {
			r = toFloat(args[0])
			g = toFloat(args[1])
			b = toFloat(args[2])
		}
		if len(args) >= 4 {
			a = toFloat(args[3])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glClearColor", r, g, b, a)
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_clear"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glClear")
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_enable_depth_test"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glEnableDepthTest")
			wasmLog("WebGL", "gl_enable_depth_test()")
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_create_shader"] = func(in *Interp, args []*Value) (*Value, error) {
		shaderType := "VERTEX"
		if len(args) > 0 {
			shaderType = args[0].String()
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			handle := bridge.Call("glCreateShader", shaderType).Int()
			wasmLog("WebGL", fmt.Sprintf("gl_create_shader(type=%s) -> id=%d", shaderType, handle))
			return IntValue(int64(handle)), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["gl_shader_source"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		shaderId := int(in.toInt(args[0]))
		src := args[1].String()
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glShaderSource", shaderId, src)
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_compile_shader"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		shaderId := int(in.toInt(args[0]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glCompileShader", shaderId)
			wasmLog("WebGL", fmt.Sprintf("gl_compile_shader(id=%d)", shaderId))
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_create_program"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			handle := bridge.Call("glCreateProgram").Int()
			wasmLog("WebGL", fmt.Sprintf("gl_create_program() -> id=%d", handle))
			return IntValue(int64(handle)), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["gl_attach_shader"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		progId := int(in.toInt(args[0]))
		shaderId := int(in.toInt(args[1]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glAttachShader", progId, shaderId)
			wasmLog("WebGL", fmt.Sprintf("gl_attach_shader(prog=%d, shader=%d)", progId, shaderId))
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_link_program"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		progId := int(in.toInt(args[0]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glLinkProgram", progId)
			wasmLog("WebGL", fmt.Sprintf("gl_link_program(prog=%d)", progId))
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_use_program"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		progId := int(in.toInt(args[0]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glUseProgram", progId)
			wasmLog("WebGL", fmt.Sprintf("gl_use_program(prog=%d)", progId))
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_get_attrib_location"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return IntValue(-1), nil
		}
		progId := int(in.toInt(args[0]))
		name := args[1].String()
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			loc := bridge.Call("glGetAttribLocation", progId, name).Int()
			wasmLog("WebGL", fmt.Sprintf("gl_get_attrib_location(prog=%d, name=%s) -> loc=%d", progId, name, loc))
			return IntValue(int64(loc)), nil
		}
		return IntValue(-1), nil
	}

	in.Builtins["gl_get_uniform_location"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return IntValue(-1), nil
		}
		progId := int(in.toInt(args[0]))
		name := args[1].String()
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			loc := bridge.Call("glGetUniformLocation", progId, name).Int()
			wasmLog("WebGL", fmt.Sprintf("gl_get_uniform_location(prog=%d, name=%s) -> loc=%d", progId, name, loc))
			return IntValue(int64(loc)), nil
		}
		return IntValue(-1), nil
	}

	in.Builtins["gl_enable_vertex_attrib_array"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		loc := int(in.toInt(args[0]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glEnableVertexAttribArray", loc)
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_create_buffer"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			handle := bridge.Call("glCreateBuffer").Int()
			wasmLog("WebGL", fmt.Sprintf("gl_create_buffer() -> id=%d", handle))
			return IntValue(int64(handle)), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["gl_bind_buffer"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		target := args[0].String()
		bufId := int(in.toInt(args[1]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glBindBuffer", target, bufId)
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_buffer_data"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		target := args[0].String()
		jsArr := js.Global().Get("Array").New()
		if args[1].Type == ValArray {
			for i, item := range args[1].ArrayVal {
				jsArr.SetIndex(i, toFloat(item))
			}
		} else {
			for i := 1; i < len(args); i++ {
				jsArr.SetIndex(i-1, toFloat(args[i]))
			}
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glBufferData", target, jsArr)
			wasmLog("WebGL", fmt.Sprintf("gl_buffer_data(target=%s, elements=%d)", target, jsArr.Length()))
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_vertex_attrib_pointer"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		loc := int(in.toInt(args[0]))
		size := int(in.toInt(args[1]))
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glVertexAttribPointer", loc, size)
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_uniform_matrix4fv"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		loc := int(in.toInt(args[0]))
		jsMat := js.Global().Get("Array").New()
		if args[1].Type == ValArray {
			for i, item := range args[1].ArrayVal {
				jsMat.SetIndex(i, toFloat(item))
			}
		} else {
			for i := 1; i < len(args); i++ {
				jsMat.SetIndex(i-1, toFloat(args[i]))
			}
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glUniformMatrix4fv", loc, jsMat)
			wasmLog("WebGL", fmt.Sprintf("gl_uniform_matrix4fv(loc=%d, elements=%d)", loc, jsMat.Length()))
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_draw_elements"] = func(in *Interp, args []*Value) (*Value, error) {
		count := 36
		if len(args) > 0 {
			count = int(in.toInt(args[0]))
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glDrawElements", count)
			wasmLog("WebGL", fmt.Sprintf("gl_draw_elements(count=%d)", count))
		}
		return BoolValue(true), nil
	}

	in.Builtins["gl_animate"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("glStartAnimation")
			wasmLog("WebGL", "gl_animate() start 60fps render loop")
		}
		return BoolValue(true), nil
	}
}

func toFloat(v *Value) float64 {
	if v == nil {
		return 0
	}
	if v.Type == ValFloat {
		return v.FloatVal
	}
	if v.Type == ValInt {
		return float64(v.IntVal)
	}
	return 0
}

func toBool(v *Value) bool {
	if v == nil {
		return false
	}
	if v.Type == ValBool {
		return v.BoolVal
	}
	if v.Type == ValInt {
		return v.IntVal != 0
	}
	return false
}
