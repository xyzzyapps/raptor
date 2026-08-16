//go:build js && wasm

package raptor

import (
	_ "embed"
	"syscall/js"
)

//go:embed embedjs/raptor_bridge.js
var raptorBridgeJS string

func ensureRaptorBridge() {
	if raptorBridgeJS == "" {
		return
	}
	b := js.Global().Get("raptorBridge")
	if !b.IsUndefined() && !b.IsNull() {
		return
	}
	js.Global().Call("eval", raptorBridgeJS)
}
