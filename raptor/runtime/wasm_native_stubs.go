//go:build !js || !wasm

package raptor

func (in *Interp) registerWebBuiltins() {
	// Fallback stubs for native desktop platforms
	in.Builtins["dom_get"] = func(in *Interp, args []*Value) (*Value, error) {
		return NilValue(), nil
	}
	in.Builtins["dom_set_text"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["dom_set_html"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["dom_create"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["canvas_init"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["canvas_clear"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["canvas_draw_rect"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["canvas_draw_circle"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["canvas_draw_line"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["canvas_draw_text"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["audio_init"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["audio_play_tone"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
	in.Builtins["audio_play_melody"] = func(in *Interp, args []*Value) (*Value, error) {
		return BoolValue(true), nil
	}
}
