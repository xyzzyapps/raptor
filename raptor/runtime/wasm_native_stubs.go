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
	in.Builtins["canvas_get_context"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	in.Builtins["canvas_set_fill_style"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_set_stroke_style"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_set_line_width"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_set_font"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_fill_rect"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_stroke_rect"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_clear_rect"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_begin_path"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_close_path"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_move_to"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_line_to"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_arc"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_stroke"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_fill"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["canvas_fill_text"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_context_create"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	in.Builtins["audio_get_current_time"] = func(in *Interp, args []*Value) (*Value, error) { return FloatValue(0.0), nil }
	in.Builtins["audio_create_oscillator"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	in.Builtins["audio_create_gain"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	in.Builtins["audio_create_biquad_filter"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	in.Builtins["audio_connect"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_connect_destination"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_set_osc_type"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_set_frequency"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_set_gain"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_gain_ramp_exp"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_gain_ramp_linear"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_set_filter_freq"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_osc_start"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["audio_osc_stop"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_init"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_clear_color"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_clear"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_enable_depth_test"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_create_shader"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	in.Builtins["gl_shader_source"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_compile_shader"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_create_program"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	in.Builtins["gl_attach_shader"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_link_program"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_use_program"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_get_attrib_location"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(0), nil }
	in.Builtins["gl_get_uniform_location"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(0), nil }
	in.Builtins["gl_enable_vertex_attrib_array"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_create_buffer"] = func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	in.Builtins["gl_bind_buffer"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_buffer_data"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_vertex_attrib_pointer"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_uniform_matrix4fv"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_draw_elements"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	in.Builtins["gl_animate"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
}
