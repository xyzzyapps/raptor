//go:build js && wasm

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"raptor/runtime"
	"syscall/js"
	"time"
)

func main() {
	c := make(chan struct{}, 0)

	js.Global().Set("raptorEval", js.FuncOf(evalHandler))
	js.Global().Set("evalRaptor", js.FuncOf(evalHandler))
	js.Global().Set("raptorWeave", js.FuncOf(weaveHandler))
	js.Global().Set("raptorTangle", js.FuncOf(tangleHandler))
	js.Global().Set("raptorStitch", js.FuncOf(stitchHandler))
	js.Global().Set("raptorVersion", js.FuncOf(versionHandler))

	fmt.Println("Raptor WebAssembly Engine Initialized Successfully!")
	<-c
}

func versionHandler(this js.Value, args []js.Value) any {
	return "Raptor v1.0.0 (WebAssembly 64-bit)"
}

func evalHandler(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		jsonBytes, _ := json.Marshal(map[string]any{"error": "no code provided"})
		return string(jsonBytes)
	}
	code := args[0].String()
	startTime := time.Now()

	var outBuf bytes.Buffer
	interp := raptor.NewInterp()
	interp.SetStdout(&outBuf)
	interp.SetStderr(&outBuf)

	val, evalErr := interp.Eval(code)
	output := outBuf.String()
	duration := time.Since(startTime).Milliseconds()

	res := map[string]any{
		"output":     output,
		"stdout":     output,
		"durationMs": duration,
	}

	if evalErr != nil {
		res["error"] = evalErr.Error()
	} else if val != nil {
		res["result"] = val.String()
	} else {
		res["result"] = "Nil"
	}

	jsonBytes, _ := json.Marshal(res)
	return string(jsonBytes)
}

func weaveHandler(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return map[string]any{"error": "no pod text provided"}
	}
	podText := args[0].String()
	doc, err := raptor.ParsePodDoc(podText)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	md := raptor.WeaveMarkdown(doc)
	return map[string]any{
		"markdown": md,
	}
}

func tangleHandler(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return map[string]any{"error": "no pod text provided"}
	}
	podText := args[0].String()
	doc, err := raptor.ParsePodDoc(podText)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	files, err := raptor.Tangle(doc, "")
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	fileMap := make(map[string]any)
	for path, content := range files {
		fileMap[path] = content
	}

	return map[string]any{
		"files": fileMap,
	}
}

func stitchHandler(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return map[string]any{"error": "expected podText and filesJson"}
	}
	podText := args[0].String()
	filesJson := args[1].String()

	var updatedFiles map[string]string
	if err := json.Unmarshal([]byte(filesJson), &updatedFiles); err != nil {
		return map[string]any{"error": fmt.Sprintf("invalid files JSON: %v", err)}
	}

	stitched, err := raptor.Stitch(podText, updatedFiles)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	return map[string]any{
		"stitchedPod": stitched,
	}
}
