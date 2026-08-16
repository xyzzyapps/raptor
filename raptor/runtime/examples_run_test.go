package raptor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamplesEval(t *testing.T) {
	dir := filepath.Join("..", "examples")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("examples dir not found")
	}
	skip := map[string]bool{
		"raylib_game.rp":     true,
		"test_raylib_ffi.rp": true,
		"esp32_blink.rp":     true,
		"esp32_i2c_display.rp": true,
		"esp32_sensor_contracts.rp": true,
		"demo_page.rphp":            true,
		"perl5_bridge.rp":           true,
		"use_raptor_module_demo.rp": true, // needs cwd at raptor/ for raptor_modules
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || skip[name] {
			continue
		}
		if !strings.HasSuffix(name, ".rp") && !strings.HasSuffix(name, ".pod") {
			continue
		}
		path := filepath.Join(dir, name)
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			src := string(b)
			if strings.HasSuffix(name, ".pod") {
				doc, err := ParsePodDoc(src)
				if err != nil {
					t.Fatal(err)
				}
				tangled, err := Tangle(doc, "")
				if err != nil {
					t.Fatal(err)
				}
				if c, ok := tangled["main.rp"]; ok {
					src = c
				} else {
					for _, v := range tangled {
						src = v
						break
					}
				}
			}
			in := NewInterp()
			var buf strings.Builder
			in.SetStdout(&buf)
			if _, err := in.Eval(src); err != nil {
				t.Fatalf("eval: %v", err)
			}
		})
	}
}
