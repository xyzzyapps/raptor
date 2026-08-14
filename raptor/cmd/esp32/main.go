//go:build embedded || tinygo || esp32 || (!js && !wasm)

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"raptor/runtime"
)

// DefaultBootScript is executed automatically when the microcontroller boots up.
const DefaultBootScript = `
# ==============================================================================
# Raptor ESP32 Microcontroller Boot Sequence
# ==============================================================================
my $STATUS_LED = 2; # Built-in LED on GPIO2
gpio_pin_mode($STATUS_LED, 1);
gpio_set($STATUS_LED, 1); # Turn ON boot LED

say "Raptor Embedded Runtime Online | Chip: " ~ chip_model() ~ " | Clock: " ~ (cpu_freq() / 1000000) ~ " MHz";
say "Free Heap Memory: " ~ (free_heap() / 1024) ~ " KB";
say "Type Raptor code or 'help' for embedded commands.";
`

func main() {
	in := raptor.NewInterp()
	in.SetStdout(os.Stdout)
	in.SetStderr(os.Stderr)

	fmt.Println("==================================================================")
	fmt.Println("  RAPTOR EMBEDDED DYNAMIC RUNTIME FOR ESP32 / MICROCONTROLLERS")
	fmt.Println("  Post-LLM Zero-Overhead Procedural Engine with Invariant Contracts")
	fmt.Println("==================================================================")

	// 1. Run Boot Script
	_, err := in.Eval(DefaultBootScript)
	if err != nil {
		fmt.Printf("Boot script error: %v\n", err)
	}

	// 2. Interactive Serial UART REPL
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("raptor> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("Serial read error: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "exit" || line == "quit" {
			fmt.Println("Restarting Raptor Embedded Session...")
			continue
		}

		if line == "help" {
			printEmbeddedHelp()
			continue
		}

		val, err := in.Eval(line)
		if err != nil {
			fmt.Printf("Runtime Error: %v\n", err)
		} else if val != nil && val.Type != raptor.ValNil {
			fmt.Printf("=> %s\n", val.String())
		}
	}
}

func printEmbeddedHelp() {
	fmt.Println("--- Raptor Embedded Hardware Peripheral Reference ---")
	fmt.Println("  gpio_pin_mode(pin, mode) : Configure GPIO pin (0: INPUT, 1: OUTPUT, 2: PULLUP)")
	fmt.Println("  gpio_set(pin, val)       : Digital write (0 or 1)")
	fmt.Println("  gpio_get(pin)            : Digital read")
	fmt.Println("  gpio_toggle(pin)         : Invert GPIO pin output state")
	fmt.Println("  analog_read(pin)         : Read analog ADC voltage (0.0..3.3V)")
	fmt.Println("  pwm_write(pin, duty)     : Set PWM duty cycle (0..255)")
	fmt.Println("  pwm_freq(pin, freq)      : Set PWM frequency in Hz")
	fmt.Println("  i2c_write(addr, @bytes)  : Transmit I2C byte packet")
	fmt.Println("  i2c_read(addr, count)    : Read I2C byte packet")
	fmt.Println("  sleep_ms(ms)             : Delay in milliseconds")
	fmt.Println("  millis()                 : Uptime in milliseconds")
	fmt.Println("  free_heap()              : Available SRAM in bytes")
	fmt.Println("-----------------------------------------------------")
}
