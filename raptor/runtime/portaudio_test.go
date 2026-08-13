package raptor

import (
	"testing"
)

func TestPortAudioEngine(t *testing.T) {
	in := NewInterp()

	code := `
my $init = pa_init();
my $verText = pa_get_version_text();
my $devCnt = pa_device_count();
my $devInfo = pa_get_device_info(0);

my $sineSamples = pa_generate_sine_wave(440.0, 0.01, 44100.0, 0.5);

pa_terminate();

[$init, $verText, $devCnt, $devInfo<name>, $sineSamples.elems];
`

	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("PortAudio eval failed: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 5 {
		t.Fatalf("expected 5 elements, got %+v", val)
	}

	if !val.ArrayVal[0].IsTrue() {
		t.Errorf("expected pa_init to succeed, got %v", val.ArrayVal[0])
	}
	if len(val.ArrayVal[1].String()) == 0 {
		t.Errorf("expected non-empty version text, got %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].IntVal < 1 {
		t.Errorf("expected at least 1 device, got %v", val.ArrayVal[2])
	}
	if val.ArrayVal[4].IntVal != 441 {
		t.Errorf("expected 441 samples for 0.01s at 44.1kHz, got %v", val.ArrayVal[4])
	}
}
