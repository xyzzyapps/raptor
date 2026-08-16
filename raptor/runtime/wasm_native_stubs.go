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
	stubTrue := func(in *Interp, args []*Value) (*Value, error) { return BoolValue(true), nil }
	stubOne := func(in *Interp, args []*Value) (*Value, error) { return IntValue(1), nil }
	stubZero := func(in *Interp, args []*Value) (*Value, error) { return FloatValue(0.0), nil }
	for _, name := range []string{
		"audio_connect", "audio_connect_destination", "audio_set_osc_type",
		"audio_set_frequency", "audio_set_gain", "audio_gain_ramp_exp",
		"audio_gain_ramp_linear", "audio_set_filter_freq", "audio_osc_start",
		"audio_osc_stop", "audio_init", "audio_play_tone", "audio_play_melody",
		"audio_set_compressor", "audio_set_delay_time", "audio_set_pan",
		"audio_set_fft_size", "audio_buffer_fill_sine", "audio_source_set_buffer",
		"audio_source_start", "audio_set_detune", "audio_set_filter_q",
		"audio_freq_ramp", "audio_connect_param", "audio_disconnect",
		"webgpu_draw_logits",
	} {
		in.Builtins[name] = stubTrue
	}
	for _, name := range []string{
		"audio_context_create", "audio_create_oscillator", "audio_create_gain",
		"audio_create_biquad_filter", "audio_create_compressor", "audio_create_delay",
		"audio_create_panner", "audio_create_analyser", "audio_create_buffer",
		"audio_create_buffer_source", "audio_destination",
	} {
		in.Builtins[name] = stubOne
	}
	in.Builtins["audio_get_current_time"] = stubZero
	in.Builtins["audio_sample_rate"] = func(in *Interp, args []*Value) (*Value, error) { return FloatValue(44100), nil }
	in.Builtins["audio_get_spectrum"] = func(in *Interp, args []*Value) (*Value, error) { return ArrayValue(nil), nil }
	in.Builtins["webgpu_init"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(false), nil }
	in.Builtins["webgpu_available"] = func(in *Interp, args []*Value) (*Value, error) { return BoolValue(false), nil }
	in.Builtins["webgpu_matmul"] = func(in *Interp, args []*Value) (*Value, error) { return ArrayValue(nil), nil }
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
