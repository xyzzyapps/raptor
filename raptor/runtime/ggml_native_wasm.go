//go:build js && wasm

package raptor

func ggmlProbeNative() (bool, string) {
	return false, ""
}

func ggmlNativeTimeUs() (int64, bool) {
	return 0, false
}
