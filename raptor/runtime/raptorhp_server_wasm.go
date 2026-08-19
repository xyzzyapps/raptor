//go:build js && wasm

package raptor

import "fmt"

// HPServerOptions is the PHP-style built-in server (`raptor -S`).
type HPServerOptions struct {
	Addr    string
	DocRoot string
	Router  string
}

func ServeRaptorHP(opts HPServerOptions) error {
	return fmt.Errorf("RaptorHP server is not available in WebAssembly")
}
