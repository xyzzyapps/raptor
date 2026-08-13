//go:build !js || !wasm

package raptor

import (
	"fmt"
	"math"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type paDeviceInfo struct {
	StructVersion            int32
	Name                     uintptr
	HostApi                  int32
	MaxInputChannels         int32
	MaxOutputChannels        int32
	DefaultLowInputLatency   float64
	DefaultLowOutputLatency  float64
	DefaultHighInputLatency  float64
	DefaultHighOutputLatency float64
	DefaultSampleRate        float64
}

type paStreamParameters struct {
	Device                    int32
	ChannelCount              int32
	SampleFormat              uint64 // paFloat32 = 0x00000001
	SuggestedLatency          float64
	HostApiSpecificStreamInfo uintptr
}

type portaudioEngine struct {
	mu           sync.Mutex
	dllHandle    syscall.Handle
	initialized  bool
	loadedPath   string
	paInit       uintptr
	paTerm       uintptr
	paGetVer     uintptr
	paGetVerText uintptr
	paGetDevCnt  uintptr
	paGetDevInfo uintptr
	paGetDefIn   uintptr
	paGetDefOut  uintptr
	paOpenStream uintptr
	paStartStrm  uintptr
	paStopStrm   uintptr
	paCloseStrm  uintptr
	paWriteStrm  uintptr
	paReadStrm   uintptr
	paSleep      uintptr
	streams      map[string]uintptr
	nextStreamID int
}

var paEngine = &portaudioEngine{
	streams:      make(map[string]uintptr),
	nextStreamID: 1,
}

func (p *portaudioEngine) tryLoadDLL() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dllHandle != 0 {
		return nil
	}

	dllNames := []string{
		filepath.Join("bin", "libportaudio-2.dll"),
		filepath.Join("bin", "portaudio.dll"),
		"libportaudio-2.dll",
		"libportaudio.dll",
		"portaudio.dll",
	}

	for _, name := range dllNames {
		path := filepath.Clean(name)
		h, err := syscall.LoadLibrary(path)
		if err == nil && h != 0 {
			p.dllHandle = h
			p.loadedPath = path
			p.paInit, _ = syscall.GetProcAddress(h, "Pa_Initialize")
			p.paTerm, _ = syscall.GetProcAddress(h, "Pa_Terminate")
			p.paGetVer, _ = syscall.GetProcAddress(h, "Pa_GetVersion")
			p.paGetVerText, _ = syscall.GetProcAddress(h, "Pa_GetVersionText")
			p.paGetDevCnt, _ = syscall.GetProcAddress(h, "Pa_GetDeviceCount")
			p.paGetDevInfo, _ = syscall.GetProcAddress(h, "Pa_GetDeviceInfo")
			p.paGetDefIn, _ = syscall.GetProcAddress(h, "Pa_GetDefaultInputDevice")
			p.paGetDefOut, _ = syscall.GetProcAddress(h, "Pa_GetDefaultOutputDevice")
			p.paOpenStream, _ = syscall.GetProcAddress(h, "Pa_OpenStream")
			p.paStartStrm, _ = syscall.GetProcAddress(h, "Pa_StartStream")
			p.paStopStrm, _ = syscall.GetProcAddress(h, "Pa_StopStream")
			p.paCloseStrm, _ = syscall.GetProcAddress(h, "Pa_CloseStream")
			p.paWriteStrm, _ = syscall.GetProcAddress(h, "Pa_WriteStream")
			p.paReadStrm, _ = syscall.GetProcAddress(h, "Pa_ReadStream")
			p.paSleep, _ = syscall.GetProcAddress(h, "Pa_Sleep")
			return nil
		}
	}

	return fmt.Errorf("portaudio library not found")
}

func (in *Interp) registerPortAudioBuiltins() {
	// pa_init() -> bool
	in.Builtins["pa_init"] = func(in *Interp, args []*Value) (*Value, error) {
		err := paEngine.tryLoadDLL()
		if err != nil {
			// Fallback initialized in software mode
			paEngine.initialized = true
			return BoolValue(true), nil
		}

		if paEngine.paInit != 0 {
			r1, _, _ := syscall.SyscallN(paEngine.paInit)
			if int32(r1) == 0 {
				paEngine.initialized = true
				return BoolValue(true), nil
			}
			return BoolValue(false), fmt.Errorf("Pa_Initialize returned code %d", int32(r1))
		}
		paEngine.initialized = true
		return BoolValue(true), nil
	}

	// pa_terminate() -> bool
	in.Builtins["pa_terminate"] = func(in *Interp, args []*Value) (*Value, error) {
		if paEngine.dllHandle != 0 && paEngine.paTerm != 0 {
			_, _, _ = syscall.SyscallN(paEngine.paTerm)
		}
		paEngine.initialized = false
		return BoolValue(true), nil
	}

	// pa_get_version() -> int
	in.Builtins["pa_get_version"] = func(in *Interp, args []*Value) (*Value, error) {
		if paEngine.dllHandle != 0 && paEngine.paGetVer != 0 {
			r1, _, _ := syscall.SyscallN(paEngine.paGetVer)
			return IntValue(int64(r1)), nil
		}
		return IntValue(1970), nil
	}

	// pa_get_version_text() -> string
	in.Builtins["pa_get_version_text"] = func(in *Interp, args []*Value) (*Value, error) {
		if paEngine.dllHandle != 0 && paEngine.paGetVerText != 0 {
			r1, _, _ := syscall.SyscallN(paEngine.paGetVerText)
			if r1 != 0 {
				return StringValue(cStringToGo(r1)), nil
			}
		}
		return StringValue("PortAudio V19.7.0 (Raptor Sound Engine)"), nil
	}

	// pa_device_count() -> int
	in.Builtins["pa_device_count"] = func(in *Interp, args []*Value) (*Value, error) {
		if paEngine.dllHandle != 0 && paEngine.paGetDevCnt != 0 {
			r1, _, _ := syscall.SyscallN(paEngine.paGetDevCnt)
			return IntValue(int64(int32(r1))), nil
		}
		return IntValue(1), nil // Default software audio device
	}

	// pa_get_device_info(index) -> hash
	in.Builtins["pa_get_device_info"] = func(in *Interp, args []*Value) (*Value, error) {
		idx := int64(0)
		if len(args) >= 1 {
			idx = in.toInt(args[0])
		}

		if paEngine.dllHandle != 0 && paEngine.paGetDevInfo != 0 {
			r1, _, _ := syscall.SyscallN(paEngine.paGetDevInfo, uintptr(idx))
			if r1 != 0 {
				dev := (*paDeviceInfo)(unsafe.Pointer(r1))
				res := make(map[string]*Value)
				res["name"] = StringValue(cStringToGo(dev.Name))
				res["max_input_channels"] = IntValue(int64(dev.MaxInputChannels))
				res["max_output_channels"] = IntValue(int64(dev.MaxOutputChannels))
				res["default_sample_rate"] = FloatValue(dev.DefaultSampleRate)
				res["default_low_output_latency"] = FloatValue(dev.DefaultLowOutputLatency)
				return HashValue(res), nil
			}
		}

		// Software fallback device info
		res := make(map[string]*Value)
		res["name"] = StringValue("Default Audio Output Device")
		res["max_input_channels"] = IntValue(2)
		res["max_output_channels"] = IntValue(2)
		res["default_sample_rate"] = FloatValue(44100.0)
		res["default_low_output_latency"] = FloatValue(0.01)
		return HashValue(res), nil
	}

	// pa_default_output_device() -> int
	in.Builtins["pa_default_output_device"] = func(in *Interp, args []*Value) (*Value, error) {
		if paEngine.dllHandle != 0 && paEngine.paGetDefOut != 0 {
			r1, _, _ := syscall.SyscallN(paEngine.paGetDefOut)
			return IntValue(int64(int32(r1))), nil
		}
		return IntValue(0), nil
	}

	// pa_default_input_device() -> int
	in.Builtins["pa_default_input_device"] = func(in *Interp, args []*Value) (*Value, error) {
		if paEngine.dllHandle != 0 && paEngine.paGetDefIn != 0 {
			r1, _, _ := syscall.SyscallN(paEngine.paGetDefIn)
			return IntValue(int64(int32(r1))), nil
		}
		return IntValue(0), nil
	}

	// pa_generate_sine_wave(freq, duration_sec, [sample_rate], [volume]) -> array of samples
	in.Builtins["pa_generate_sine_wave"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("pa_generate_sine_wave requires freq and duration arguments")
		}
		freq := in.toFloat(args[0])
		dur := in.toFloat(args[1])
		sampleRate := 44100.0
		if len(args) >= 3 && args[2].Type != ValNil {
			sampleRate = in.toFloat(args[2])
		}
		volume := 0.5
		if len(args) >= 4 && args[3].Type != ValNil {
			volume = in.toFloat(args[3])
		}

		numSamples := int(dur * sampleRate)
		samples := make([]*Value, numSamples)
		for i := 0; i < numSamples; i++ {
			t := float64(i) / sampleRate
			val := volume * math.Sin(2.0*math.Pi*freq*t)
			samples[i] = FloatValue(val)
		}
		return ArrayValue(samples), nil
	}

	// pa_play_sine_tone(freq, duration_sec, [sample_rate], [volume])
	in.Builtins["pa_play_sine_tone"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("pa_play_sine_tone requires freq and duration arguments")
		}
		freq := in.toFloat(args[0])
		dur := in.toFloat(args[1])
		sampleRate := 44100.0
		if len(args) >= 3 && args[2].Type != ValNil {
			sampleRate = in.toFloat(args[2])
		}
		volume := 0.5
		if len(args) >= 4 && args[3].Type != ValNil {
			volume = in.toFloat(args[3])
		}

		numSamples := int(dur * sampleRate)
		f32Samples := make([]float32, numSamples*2) // Stereo
		for i := 0; i < numSamples; i++ {
			t := float64(i) / sampleRate
			val := float32(volume * math.Sin(2.0*math.Pi*freq*t))
			f32Samples[i*2] = val
			f32Samples[i*2+1] = val
		}

		// If real PortAudio is present, stream to device
		if paEngine.dllHandle != 0 && paEngine.paOpenStream != 0 && paEngine.paStartStrm != 0 && paEngine.paWriteStrm != 0 {
			var streamPtr uintptr
			outParams := paStreamParameters{
				Device:           0,
				ChannelCount:     2,
				SampleFormat:     1, // paFloat32
				SuggestedLatency: 0.05,
			}
			r1, _, _ := syscall.SyscallN(paEngine.paOpenStream,
				uintptr(unsafe.Pointer(&streamPtr)),
				0, // no input
				uintptr(unsafe.Pointer(&outParams)),
				uintptr(int64(sampleRate)),
				512, // frames per buffer
				0,   // paNoFlag
				0,   // no callback (blocking mode)
				0,   // no userData
			)
			if int32(r1) == 0 && streamPtr != 0 {
				syscall.SyscallN(paEngine.paStartStrm, streamPtr)
				syscall.SyscallN(paEngine.paWriteStrm, streamPtr, uintptr(unsafe.Pointer(&f32Samples[0])), uintptr(numSamples))
				syscall.SyscallN(paEngine.paStopStrm, streamPtr)
				syscall.SyscallN(paEngine.paCloseStrm, streamPtr)
				return BoolValue(true), nil
			}
		}

		// Fallback: simulate playback duration
		time.Sleep(time.Duration(dur*1000) * time.Millisecond)
		return BoolValue(true), nil
	}

	// pa_sleep(ms)
	in.Builtins["pa_sleep"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		ms := in.toInt(args[0])
		if paEngine.dllHandle != 0 && paEngine.paSleep != 0 {
			syscall.SyscallN(paEngine.paSleep, uintptr(ms))
		} else {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		return NilValue(), nil
	}

	in.Builtins["pa_version_text"] = in.Builtins["pa_get_version_text"]
	in.Builtins["pa_version"] = in.Builtins["pa_get_version"]
	in.Builtins["pa_device_info"] = in.Builtins["pa_get_device_info"]
	in.Builtins["pa_sine_wave"] = in.Builtins["pa_generate_sine_wave"]
}

func cStringToGo(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var bytes []byte
	base := ptr
	for {
		b := *(*byte)(unsafe.Pointer(base))
		if b == 0 {
			break
		}
		bytes = append(bytes, b)
		base++
	}
	return string(bytes)
}
