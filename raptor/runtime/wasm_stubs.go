//go:build js && wasm

package raptor

import (
	"fmt"
	"syscall/js"
)

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

	in.Builtins["canvas_draw_rect"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 5 {
			return BoolValue(false), nil
		}
		x := toFloat(args[0])
		y := toFloat(args[1])
		w := toFloat(args[2])
		h := toFloat(args[3])
		color := args[4].String()
		fill := true
		if len(args) > 5 {
			fill = toBool(args[5])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("drawRect", x, y, w, h, color, fill)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_draw_circle"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 4 {
			return BoolValue(false), nil
		}
		x := toFloat(args[0])
		y := toFloat(args[1])
		r := toFloat(args[2])
		color := args[3].String()
		fill := true
		if len(args) > 4 {
			fill = toBool(args[4])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("drawCircle", x, y, r, color, fill)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_draw_line"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 5 {
			return BoolValue(false), nil
		}
		x1 := toFloat(args[0])
		y1 := toFloat(args[1])
		x2 := toFloat(args[2])
		y2 := toFloat(args[3])
		color := args[4].String()
		lw := 1.0
		if len(args) > 5 {
			lw = toFloat(args[5])
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("drawLine", x1, y1, x2, y2, color, lw)
		}
		return BoolValue(true), nil
	}

	in.Builtins["canvas_draw_text"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 4 {
			return BoolValue(false), nil
		}
		text := args[0].String()
		x := toFloat(args[1])
		y := toFloat(args[2])
		size := int(args[3].IntVal)
		if size <= 0 {
			size = 14
		}
		color := "#ffffff"
		if len(args) > 4 {
			color = args[4].String()
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("drawText", text, x, y, size, color)
		}
		return BoolValue(true), nil
	}

	// WebAudio API
	in.Builtins["audio_init"] = func(in *Interp, args []*Value) (*Value, error) {
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("initAudio")
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
		waveform := "sine"
		if len(args) > 2 {
			waveform = args[2].String()
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("playTone", freq, dur, waveform)
		}
		return BoolValue(true), nil
	}

	in.Builtins["audio_play_melody"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return BoolValue(false), nil
		}
		var freqs []any
		if args[0].Type == ValArray {
			for _, item := range args[0].ArrayVal {
				freqs = append(freqs, toFloat(item))
			}
		}
		var durs []any
		if len(args) > 1 && args[1].Type == ValArray {
			for _, item := range args[1].ArrayVal {
				durs = append(durs, toFloat(item))
			}
		}
		bridge := js.Global().Get("raptorBridge")
		if !bridge.IsNull() && !bridge.IsUndefined() {
			bridge.Call("playMelody", freqs, durs)
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
