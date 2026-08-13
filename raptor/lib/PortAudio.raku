# ==============================================================================
# PortAudio Procedural Sound Engine Bindings for Raptor
# Strictly Procedural / Functional (No OO classes)
# ==============================================================================

struct PaDeviceInfo {
    int32 $structVersion;
    Str $name;
    int32 $hostApi;
    int32 $maxInputChannels;
    int32 $maxOutputChannels;
    num64 $defaultLowInputLatency;
    num64 $defaultLowOutputLatency;
    num64 $defaultHighInputLatency;
    num64 $defaultHighOutputLatency;
    num64 $defaultSampleRate;
}

sub audio_init() {
    return pa_init();
}

sub audio_terminate() {
    return pa_terminate();
}

sub audio_version() {
    return pa_get_version_text();
}

sub audio_devices() {
    my $cnt = pa_device_count();
    my @devs;
    for 0..($cnt - 1) -> $i {
        push(@devs, pa_get_device_info($i));
    }
    return @devs;
}

sub audio_play_tone(num64 $freq, num64 $duration_sec, num64 $sample_rate = 44100.0, num64 $volume = 0.5) {
    return pa_play_sine_tone($freq, $duration_sec, $sample_rate, $volume);
}

sub audio_synth_wave(num64 $freq, num64 $duration_sec, num64 $sample_rate = 44100.0, num64 $volume = 0.5) {
    return pa_generate_sine_wave($freq, $duration_sec, $sample_rate, $volume);
}

sub audio_wait(Int $ms) {
    pa_sleep($ms);
}
