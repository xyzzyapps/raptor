package raptor

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmbeddedGPIOBuiltins(t *testing.T) {
	hw := GetHardwareState()
	hw.Reset()

	code := `
		# Set pin 2 as OUTPUT
		gpio_pin_mode(2, 1);
		
		# Turn LED ON
		gpio_set(2, 1);
		my $val1 = gpio_get(2);
		
		# Toggle LED
		gpio_toggle(2);
		my $val2 = gpio_get(2);
		
		say "VAL1=" ~ $val1 ~ " VAL2=" ~ $val2;
	`
	in := NewInterp()
	var outBuf bytes.Buffer
	in.SetStdout(&outBuf)

	_, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Embedded GPIO eval failed: %v", err)
	}

	out := strings.TrimSpace(outBuf.String())
	if !strings.Contains(out, "VAL1=1 VAL2=0") {
		t.Errorf("Unexpected GPIO output: %s", out)
	}

	if hw.PinModes[2] != 1 {
		t.Errorf("Expected Pin 2 mode = 1, got: %d", hw.PinModes[2])
	}
	if hw.PinValues[2] != false {
		t.Errorf("Expected Pin 2 value = false, got: %v", hw.PinValues[2])
	}
}

func TestEmbeddedAnalogAndPWMBuiltins(t *testing.T) {
	hw := GetHardwareState()
	hw.Reset()
	hw.AnalogPins[34] = 2.45 // Mock ADC voltage on GPIO34

	code := `
		my $voltage = analog_read(34);
		pwm_freq(18, 5000);
		pwm_write(18, 128);
		
		say "VOLTAGE=" ~ $voltage;
	`
	in := NewInterp()
	var outBuf bytes.Buffer
	in.SetStdout(&outBuf)

	_, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Embedded Analog/PWM eval failed: %v", err)
	}

	out := strings.TrimSpace(outBuf.String())
	if !strings.Contains(out, "VOLTAGE=2.45") {
		t.Errorf("Unexpected Analog output: %s", out)
	}

	if hw.PWMFreqs[18] != 5000 {
		t.Errorf("Expected PWM freq = 5000, got: %d", hw.PWMFreqs[18])
	}
	if hw.PWMDuties[18] != 128 {
		t.Errorf("Expected PWM duty = 128, got: %d", hw.PWMDuties[18])
	}
}

func TestEmbeddedI2CCommunication(t *testing.T) {
	hw := GetHardwareState()
	hw.Reset()
	hw.I2CReads[0x3C] = []byte{0x40, 0xFF, 0x00} // Mock SSD1306 OLED display data

	code := `
		my @cmd = [0x00, 0xAF]; # Display ON command
		my $bytes_sent = i2c_write(0x3C, @cmd);
		
		my @read_bytes = i2c_read(0x3C, 3);
		
		say "SENT=" ~ $bytes_sent ~ " READ0=" ~ @read_bytes[0];
	`
	in := NewInterp()
	var outBuf bytes.Buffer
	in.SetStdout(&outBuf)

	_, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Embedded I2C eval failed: %v", err)
	}

	out := strings.TrimSpace(outBuf.String())
	if !strings.Contains(out, "SENT=2 READ0=64") { // 0x40 = 64
		t.Errorf("Unexpected I2C output: %s", out)
	}

	if len(hw.I2CBuses[0x3C]) != 2 || hw.I2CBuses[0x3C][1] != 0xAF {
		t.Errorf("I2C buffer write mismatch: %v", hw.I2CBuses[0x3C])
	}
}

func TestEmbeddedHardwareContracts(t *testing.T) {
	hw := GetHardwareState()
	hw.Reset()

	code := `
		subset ValidDuty of Int where { $_ >= 0 && $_ <= 255 };
		subset SafeTemp of Num where { $_ >= -20.0 && $_ <= 85.0 };

		sub set_actuator(ValidDuty $duty) {
			pwm_write(25, $duty);
			return $duty;
		}

		sub verify_temp(SafeTemp $t) {
			return $t;
		}

		set_actuator(200);
		verify_temp(42.5);
		say "CONTRACTS VERIFIED";
	`
	in := NewInterp()
	var outBuf bytes.Buffer
	in.SetStdout(&outBuf)

	_, err := in.Eval(code)
	if err != nil {
		t.Fatalf("Hardware contract eval failed: %v", err)
	}

	out := strings.TrimSpace(outBuf.String())
	if !strings.Contains(out, "CONTRACTS VERIFIED") {
		t.Errorf("Expected 'CONTRACTS VERIFIED', got: %s", out)
	}

	// Test invalid duty contract rejection
	badCode := `
		subset ValidDuty of Int where { $_ >= 0 && $_ <= 255 };
		sub set_actuator(ValidDuty $duty) {
			pwm_write(25, $duty);
		}
		set_actuator(300); # Out of range duty
	`
	inBad := NewInterp()
	_, badErr := inBad.Eval(badCode)
	if badErr == nil {
		t.Errorf("Expected contract violation for duty 300, but succeeded")
	}
}
