package raptor

import (
	"sync"
	"time"
)

// HardwareState stores mock/emulated hardware state when running on host or bridge to TinyGo.
type HardwareState struct {
	mu         sync.RWMutex
	PinModes   map[int]int       // pin -> mode (0: INPUT, 1: OUTPUT, 2: INPUT_PULLUP)
	PinValues  map[int]bool      // pin -> digital state
	PWMDuties  map[int]int       // pin -> PWM duty cycle (0..255 or 0..1023)
	PWMFreqs   map[int]int       // pin -> PWM frequency in Hz
	AnalogPins map[int]float64   // pin -> analog voltage/value (0.0 .. 3.3V or 0..4095)
	I2CBuses   map[int][]byte    // addr -> last written I2C payload
	I2CReads   map[int][]byte    // addr -> mock data to return on read
	StartTime  time.Time
}

var globalHW = &HardwareState{
	PinModes:   make(map[int]int),
	PinValues:  make(map[int]bool),
	PWMDuties:  make(map[int]int),
	PWMFreqs:   make(map[int]int),
	AnalogPins: make(map[int]float64),
	I2CBuses:   make(map[int][]byte),
	I2CReads:   make(map[int][]byte),
	StartTime:  time.Now(),
}

// GetHardwareState returns the singleton hardware state tracker.
func GetHardwareState() *HardwareState {
	return globalHW
}

// Reset clears simulated hardware state.
func (hw *HardwareState) Reset() {
	hw.mu.Lock()
	defer hw.mu.Unlock()
	hw.PinModes = make(map[int]int)
	hw.PinValues = make(map[int]bool)
	hw.PWMDuties = make(map[int]int)
	hw.PWMFreqs = make(map[int]int)
	hw.AnalogPins = make(map[int]float64)
	hw.I2CBuses = make(map[int][]byte)
	hw.I2CReads = make(map[int][]byte)
	hw.StartTime = time.Now()
}
