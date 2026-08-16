//go:build js && wasm

package raptor

import (
	"fmt"
	"syscall/js"
	"time"
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

func (in *Interp) registerHTTPBuiltins() {
	in.Builtins["http_get"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("HTTP client/server sockets are not supported directly in WASM (use Fetch API)")
	}
	in.Builtins["http_listen"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("HTTP listener is not supported in browser WebAssembly")
	}
}

func (in *Interp) registerWebSocketBuiltins() {
	in.Builtins["ws_connect"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("Raw WebSockets not supported directly in WASM sandbox")
	}
	in.Builtins["ws_listen"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("WebSocket server not supported in browser WebAssembly")
	}
}

func (in *Interp) registerSocketBuiltins() {
	in.Builtins["socket_listen"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("TCP/UDP raw sockets are not available in WebAssembly sandbox")
	}
	in.Builtins["socket_connect"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("TCP/UDP raw sockets are not available in WebAssembly sandbox")
	}
}

func (in *Interp) registerPerl5Bridge() {
	in.Builtins["eval_perl5"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("Perl5 process bridge is not supported in WebAssembly sandbox")
	}
	in.Builtins["call_perl5"] = func(in *Interp, args []*Value) (*Value, error) {
		return nil, fmt.Errorf("Perl5 process bridge is not supported in WebAssembly sandbox")
	}
}

func (in *Interp) evalUse(u *UseStmt, env *Env) (*Value, error) {
	return nil, fmt.Errorf("module 'use' with external Perl5 modules is not supported in WebAssembly environment")
}

func (in *Interp) registerWebBuiltins() {
	ensureRaptorBridge()

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
		if b, ok := wasmBridge(); ok {
			b.Call("audioOscStop", oscId, stopTime)
			wasmLog("WebAudio", fmt.Sprintf("audio_osc_stop(osc=%d, stopTime=%.3f)", oscId, stopTime))
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_create_compressor"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		if b, ok := wasmBridge(); ok {
			id := b.Call("audioCreateCompressor", ctxId).Int()
			return IntValue(int64(id)), nil
		}
		return IntValue(0), nil
	}
	in.Builtins["audio_set_compressor"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		id := int(in.toInt(args[0]))
		thr, knee, ratio, atk, rel := -24.0, 30.0, 12.0, 0.003, 0.25
		if len(args) > 1 {
			thr = toFloat(args[1])
		}
		if len(args) > 2 {
			knee = toFloat(args[2])
		}
		if len(args) > 3 {
			ratio = toFloat(args[3])
		}
		if len(args) > 4 {
			atk = toFloat(args[4])
		}
		if len(args) > 5 {
			rel = toFloat(args[5])
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioSetCompressor", id, thr, knee, ratio, atk, rel)
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_create_delay"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		maxD := 1.0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		if len(args) > 1 {
			maxD = toFloat(args[1])
		}
		if b, ok := wasmBridge(); ok {
			return IntValue(int64(b.Call("audioCreateDelay", ctxId, maxD).Int())), nil
		}
		return IntValue(0), nil
	}
	in.Builtins["audio_set_delay_time"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		t := 0.0
		if len(args) > 2 {
			t = toFloat(args[2])
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioSetDelayTime", int(in.toInt(args[0])), toFloat(args[1]), t)
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_create_panner"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		if b, ok := wasmBridge(); ok {
			return IntValue(int64(b.Call("audioCreatePanner", ctxId).Int())), nil
		}
		return IntValue(0), nil
	}
	in.Builtins["audio_set_pan"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		t := 0.0
		if len(args) > 2 {
			t = toFloat(args[2])
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioSetPan", int(in.toInt(args[0])), toFloat(args[1]), t)
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_create_analyser"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		if b, ok := wasmBridge(); ok {
			return IntValue(int64(b.Call("audioCreateAnalyser", ctxId).Int())), nil
		}
		return IntValue(0), nil
	}
	in.Builtins["audio_set_fft_size"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioSetFftSize", int(in.toInt(args[0])), int(in.toInt(args[1])))
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_get_spectrum"] = func(in *Interp, args []*Value) (*Value, error) {
		id := 0
		if len(args) > 0 {
			id = int(in.toInt(args[0]))
		}
		if b, ok := wasmBridge(); ok {
			arr := b.Call("audioGetSpectrum", id)
			n := arr.Length()
			out := make([]*Value, n)
			for i := 0; i < n; i++ {
				out[i] = FloatValue(arr.Index(i).Float())
			}
			return ArrayValue(out), nil
		}
		return ArrayValue(nil), nil
	}
	in.Builtins["audio_create_buffer"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId, ch, length, sr := 0, 1, 44100, 44100.0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		if len(args) > 1 {
			ch = int(in.toInt(args[1]))
		}
		if len(args) > 2 {
			length = int(in.toInt(args[2]))
		}
		if len(args) > 3 {
			sr = toFloat(args[3])
		}
		if b, ok := wasmBridge(); ok {
			return IntValue(int64(b.Call("audioCreateBuffer", ctxId, ch, length, sr).Int())), nil
		}
		return IntValue(0), nil
	}
	in.Builtins["audio_buffer_fill_sine"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioBufferFillSine", int(in.toInt(args[0])), toFloat(args[1]))
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_create_buffer_source"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		if b, ok := wasmBridge(); ok {
			return IntValue(int64(b.Call("audioCreateBufferSource", ctxId).Int())), nil
		}
		return IntValue(0), nil
	}
	in.Builtins["audio_source_set_buffer"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioSourceSetBuffer", int(in.toInt(args[0])), int(in.toInt(args[1])))
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_source_start"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		t := 0.0
		if len(args) > 1 {
			t = toFloat(args[1])
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioSourceStart", int(in.toInt(args[0])), t)
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_set_detune"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		t := 0.0
		if len(args) > 2 {
			t = toFloat(args[2])
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioSetDetune", int(in.toInt(args[0])), toFloat(args[1]), t)
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_set_filter_q"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return BoolValue(false), nil
		}
		t := 0.0
		if len(args) > 2 {
			t = toFloat(args[2])
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioSetFilterQ", int(in.toInt(args[0])), toFloat(args[1]), t)
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_freq_ramp"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return BoolValue(false), nil
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioFreqRamp", int(in.toInt(args[0])), toFloat(args[1]), toFloat(args[2]))
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_connect_param"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 3 {
			return BoolValue(false), nil
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioConnectParam", int(in.toInt(args[0])), int(in.toInt(args[1])), args[2].String())
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_disconnect"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		if b, ok := wasmBridge(); ok {
			b.Call("audioDisconnect", int(in.toInt(args[0])))
		}
		return BoolValue(true), nil
	}
	in.Builtins["audio_destination"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		if b, ok := wasmBridge(); ok {
			return IntValue(int64(b.Call("audioDestination", ctxId).Int())), nil
		}
		return IntValue(0), nil
	}
	in.Builtins["audio_sample_rate"] = func(in *Interp, args []*Value) (*Value, error) {
		ctxId := 0
		if len(args) > 0 {
			ctxId = int(in.toInt(args[0]))
		}
		if b, ok := wasmBridge(); ok {
			return FloatValue(b.Call("audioSampleRate", ctxId).Float()), nil
		}
		return FloatValue(44100), nil
	}

	in.Builtins["audio_init"] = func(in *Interp, args []*Value) (*Value, error) {
		if b, ok := wasmBridge(); ok {
			b.Call("initAudio")
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
		if b, ok := wasmBridge(); ok {
			// Build a one-shot osc -> gain -> dest on the AudioContext timeline.
			ctx := b.Call("audioContextCreate").Int()
			t0 := b.Call("audioGetCurrentTime", ctx).Float()
			osc := b.Call("audioCreateOscillator", ctx).Int()
			gain := b.Call("audioCreateGain", ctx).Int()
			b.Call("audioSetOscType", osc, waveform)
			b.Call("audioSetFrequency", osc, freq, t0)
			b.Call("audioSetGain", gain, 0.18, t0)
			b.Call("audioGainRampExp", gain, 0.0001, t0+dur)
			b.Call("audioConnect", osc, gain)
			b.Call("audioConnectDestination", gain, ctx)
			b.Call("audioOscStart", osc, t0)
			b.Call("audioOscStop", osc, t0+dur)
			wasmLog("WebAudio", fmt.Sprintf("audio_play_tone(freq=%.2f, dur=%.2f, wave=%s)", freq, dur, waveform))
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_play_melody"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 || args[0].Type != ValArray {
			return BoolValue(false), nil
		}
		b, ok := wasmBridge()
		if !ok {
			return BoolValue(false), nil
		}
		ctx := b.Call("audioContextCreate").Int()
		t0 := b.Call("audioGetCurrentTime", ctx).Float() + 0.03
		filter := b.Call("audioCreateBiquadFilter", ctx, "lowpass").Int()
		master := b.Call("audioCreateGain", ctx).Int()
		comp := b.Call("audioCreateCompressor", ctx).Int()
		b.Call("audioSetFilterFreq", filter, 2200.0, t0)
		b.Call("audioSetFilterQ", filter, 2.5, t0)
		b.Call("audioSetGain", master, 0.2, t0)
		b.Call("audioSetCompressor", comp, -18.0, 12.0, 4.0, 0.003, 0.12)
		b.Call("audioConnect", filter, master)
		b.Call("audioConnect", master, comp)
		b.Call("audioConnectDestination", comp, ctx)
		t := t0
		for i, item := range args[0].ArrayVal {
			freq := toFloat(item)
			dur := 0.18
			if len(args) > 1 && args[1].Type == ValArray && i < len(args[1].ArrayVal) {
				dur = toFloat(args[1].ArrayVal[i])
			}
			osc := b.Call("audioCreateOscillator", ctx).Int()
			env := b.Call("audioCreateGain", ctx).Int()
			b.Call("audioSetOscType", osc, "triangle")
			b.Call("audioSetFrequency", osc, freq, t)
			b.Call("audioSetGain", env, 0.0001, t)
			b.Call("audioGainRampExp", env, 0.8, t+0.015)
			b.Call("audioGainRampExp", env, 0.0001, t+dur)
			b.Call("audioConnect", osc, env)
			b.Call("audioConnect", env, filter)
			b.Call("audioOscStart", osc, t)
			b.Call("audioOscStop", osc, t+dur+0.02)
			t += dur
		}
		wasmLog("WebAudio", fmt.Sprintf("audio_play_melody(notes=%d) via node graph", len(args[0].ArrayVal)))
		return BoolValue(true), nil
	}

	registerWasmWebGPU(in)

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

func jsTimeout(ms int) <-chan time.Time {
	return time.After(time.Duration(ms) * time.Millisecond)
}

func wasmBridge() (js.Value, bool) {
	b := js.Global().Get("raptorBridge")
	if b.IsNull() || b.IsUndefined() {
		return b, false
	}
	return b, true
}

func jsFloatArray(v js.Value) []float32 {
	n := v.Length()
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = float32(v.Index(i).Float())
	}
	return out
}

func toJSFloatArray(xs []float32) js.Value {
	arr := js.Global().Get("Array").New(len(xs))
	for i, v := range xs {
		arr.SetIndex(i, v)
	}
	return arr
}

func registerWasmWebGPU(in *Interp) {
	tinyLMMatmulHook = func(m, n, k int, a, b []float32) []float32 {
		br, ok := wasmBridge()
		if !ok {
			return nil
		}
		if !br.Get("webgpuReady").Truthy() {
			return nil
		}
		ch := make(chan []float32, 1)
		cb := js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) > 0 {
				ch <- jsFloatArray(args[0])
			} else {
				ch <- nil
			}
			return nil
		})
		defer cb.Release()
		br.Call("webgpuMatmulAsync", m, n, k, toJSFloatArray(a), toJSFloatArray(b), cb)
		select {
		case out := <-ch:
			if len(out) == m*n {
				return out
			}
		case <-jsTimeout(2000):
		}
		return nil
	}

	in.Builtins["webgpu_init"] = func(in *Interp, args []*Value) (*Value, error) {
		id := "wasmWebGPUCanvas"
		w, h := 640, 320
		if len(args) > 0 {
			id = args[0].String()
		}
		if len(args) > 2 {
			w = int(in.toInt(args[1]))
			h = int(in.toInt(args[2]))
		}
		if b, ok := wasmBridge(); ok {
			okv := b.Call("webgpuInit", id, w, h)
			wasmLog("WebGPU", fmt.Sprintf("webgpu_init(%s, %d, %d) -> %v", id, w, h, okv))
			if okv.Truthy() {
				return BoolValue(true), nil
			}
		}
		return BoolValue(false), nil
	}

	in.Builtins["webgpu_available"] = func(in *Interp, args []*Value) (*Value, error) {
		if b, ok := wasmBridge(); ok {
			return BoolValue(b.Call("webgpuAvailable").Truthy()), nil
		}
		return BoolValue(false), nil
	}

	in.Builtins["webgpu_matmul"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 5 || args[3].Type != ValArray || args[4].Type != ValArray {
			return ArrayValue(nil), fmt.Errorf("webgpu_matmul(m, n, k, a, b)")
		}
		m := int(in.toInt(args[0]))
		n := int(in.toInt(args[1]))
		k := int(in.toInt(args[2]))
		a := make([]float32, len(args[3].ArrayVal))
		b := make([]float32, len(args[4].ArrayVal))
		for i, v := range args[3].ArrayVal {
			a[i] = float32(toFloat(v))
		}
		for i, v := range args[4].ArrayVal {
			b[i] = float32(toFloat(v))
		}
		out := tinyLMMatmul(m, n, k, a, b)
		res := make([]*Value, len(out))
		for i, v := range out {
			res[i] = FloatValue(float64(v))
		}
		return ArrayValue(res), nil
	}

	in.Builtins["webgpu_draw_logits"] = func(in *Interp, args []*Value) (*Value, error) {
		var logits []float32
		if len(args) > 0 && args[0].Type == ValArray {
			logits = make([]float32, len(args[0].ArrayVal))
			for i, v := range args[0].ArrayVal {
				logits[i] = float32(toFloat(v))
			}
		}
		if b, ok := wasmBridge(); ok {
			b.Call("webgpuDrawLogits", toJSFloatArray(logits), tinyLMVocabSrc)
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
