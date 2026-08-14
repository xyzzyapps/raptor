package raptor

import (
	"fmt"
	"time"
)

// registerEmbeddedBuiltins registers hardware peripheral functions for microcontrollers and embedded targets.
func (in *Interp) registerEmbeddedBuiltins() {
	hw := GetHardwareState()

	// --- GPIO Digital I/O ---
	in.Builtins["gpio_pin_mode"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("gpio_pin_mode requires pin and mode (0: INPUT, 1: OUTPUT, 2: PULLUP)")
		}
		pin := int(in.toInt(args[0]))
		mode := int(in.toInt(args[1]))
		hw.mu.Lock()
		hw.PinModes[pin] = mode
		hw.mu.Unlock()
		return BoolValue(true), nil
	}

	in.Builtins["gpio_set"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("gpio_set requires pin and value (0 or 1)")
		}
		pin := int(in.toInt(args[0]))
		val := args[1].IsTrue()
		hw.mu.Lock()
		hw.PinValues[pin] = val
		hw.mu.Unlock()
		return BoolValue(true), nil
	}

	in.Builtins["gpio_get"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("gpio_get requires pin number")
		}
		pin := int(in.toInt(args[0]))
		hw.mu.RLock()
		val := hw.PinValues[pin]
		hw.mu.RUnlock()
		if val {
			return IntValue(1), nil
		}
		return IntValue(0), nil
	}

	in.Builtins["gpio_toggle"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("gpio_toggle requires pin number")
		}
		pin := int(in.toInt(args[0]))
		hw.mu.Lock()
		curr := hw.PinValues[pin]
		hw.PinValues[pin] = !curr
		hw.mu.Unlock()
		if !curr {
			return IntValue(1), nil
		}
		return IntValue(0), nil
	}

	// --- Analog ADC & PWM ---
	in.Builtins["analog_read"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("analog_read requires pin number")
		}
		pin := int(in.toInt(args[0]))
		hw.mu.RLock()
		val, ok := hw.AnalogPins[pin]
		hw.mu.RUnlock()
		if !ok {
			val = 0.0
		}
		return FloatValue(val), nil
	}

	in.Builtins["pwm_write"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("pwm_write requires pin and duty cycle (0..255)")
		}
		pin := int(in.toInt(args[0]))
		duty := int(in.toInt(args[1]))
		hw.mu.Lock()
		hw.PWMDuties[pin] = duty
		hw.mu.Unlock()
		return BoolValue(true), nil
	}

	in.Builtins["pwm_freq"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("pwm_freq requires pin and frequency in Hz")
		}
		pin := int(in.toInt(args[0]))
		freq := int(in.toInt(args[1]))
		hw.mu.Lock()
		hw.PWMFreqs[pin] = freq
		hw.mu.Unlock()
		return BoolValue(true), nil
	}

	// --- I2C / SPI Communication Buses ---
	in.Builtins["i2c_write"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("i2c_write requires address and data array")
		}
		addr := int(in.toInt(args[0]))
		var buf []byte
		if args[1].Type == ValArray {
			for _, b := range args[1].ArrayVal {
				buf = append(buf, byte(in.toInt(b)))
			}
		} else {
			for i := 1; i < len(args); i++ {
				buf = append(buf, byte(in.toInt(args[i])))
			}
		}
		hw.mu.Lock()
		hw.I2CBuses[addr] = buf
		hw.mu.Unlock()
		return IntValue(int64(len(buf))), nil
	}

	in.Builtins["i2c_read"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("i2c_read requires address and byte count")
		}
		addr := int(in.toInt(args[0]))
		count := int(in.toInt(args[1]))
		hw.mu.RLock()
		preset, ok := hw.I2CReads[addr]
		hw.mu.RUnlock()

		var out []*Value
		for i := 0; i < count; i++ {
			if ok && i < len(preset) {
				out = append(out, IntValue(int64(preset[i])))
			} else {
				out = append(out, IntValue(0))
			}
		}
		return ArrayValue(out), nil
	}

	in.Builtins["spi_transfer"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("spi_transfer requires data array or byte")
		}
		var out []*Value
		if args[0].Type == ValArray {
			for _, b := range args[0].ArrayVal {
				out = append(out, b)
			}
		} else {
			for _, a := range args {
				out = append(out, a)
			}
		}
		return ArrayValue(out), nil
	}

	// --- Timing & Delays ---
	in.Builtins["millis"] = func(in *Interp, args []*Value) (*Value, error) {
		ms := time.Since(hw.StartTime).Milliseconds()
		return IntValue(ms), nil
	}

	in.Builtins["micros"] = func(in *Interp, args []*Value) (*Value, error) {
		us := time.Since(hw.StartTime).Microseconds()
		return IntValue(us), nil
	}

	in.Builtins["sleep_ms"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		ms := in.toInt(args[0])
		if ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		return NilValue(), nil
	}

	in.Builtins["sleep_us"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return NilValue(), nil
		}
		us := in.toInt(args[0])
		if us > 0 {
			time.Sleep(time.Duration(us) * time.Microsecond)
		}
		return NilValue(), nil
	}

	// --- Machine Hardware Telemetry ---
	in.Builtins["cpu_freq"] = func(in *Interp, args []*Value) (*Value, error) {
		return IntValue(240000000), nil // 240 MHz standard ESP32 clock
	}

	in.Builtins["free_heap"] = func(in *Interp, args []*Value) (*Value, error) {
		return IntValue(327680), nil // ~320 KB free heap
	}

	in.Builtins["chip_model"] = func(in *Interp, args []*Value) (*Value, error) {
		return StringValue("ESP32-DualCore-240MHz"), nil
	}
}
