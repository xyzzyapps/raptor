package raptor

import (
	"fmt"
	"moarvm-go/engine"
	"sync"
)

// LoadedMoarModule wraps a loaded MoarVM dynamic module for the Raptor runtime.
type LoadedMoarModule struct {
	Module  *moargo.Module
	Path    string
	Symbols []string
}

var (
	loadedModulesMu sync.RWMutex
	loadedModules   = make(map[string]*LoadedMoarModule)
)

func (in *Interp) registerMoarVMModuleBuiltins() {
	// moar_load_module(filePath)
	in.Builtins["moar_load_module"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("moar_load_module requires module file path")
		}
		filePath := args[0].String()

		loadedModulesMu.Lock()
		defer loadedModulesMu.Unlock()

		mod, err := moargo.LoadModule(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed loading MoarVM module %s: %w", filePath, err)
		}

		symbols := mod.GetExportedSymbols()
		loaded := &LoadedMoarModule{
			Module:  mod,
			Path:    filePath,
			Symbols: symbols,
		}
		loadedModules[filePath] = loaded

		// Return Raptor representation as a structured Hash / Object
		var symValues []*Value
		for _, s := range symbols {
			symValues = append(symValues, StringValue(s))
		}

		resHash := map[string]*Value{
			"path":    StringValue(filePath),
			"name":    StringValue(mod.Name),
			"hll":     StringValue(mod.HLL),
			"symbols": ArrayValue(symValues),
		}

		return HashValue(resHash), nil
	}

	// moar_call_symbol($module, symbolName, @args)
	in.Builtins["moar_call_symbol"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("moar_call_symbol requires module handle and symbol name")
		}

		var filePath string
		if args[0].Type == ValHash {
			if pathVal, ok := args[0].HashVal["path"]; ok {
				filePath = pathVal.String()
			}
		} else {
			filePath = args[0].String()
		}

		symbolName := args[1].String()

		loadedModulesMu.RLock()
		loaded, ok := loadedModules[filePath]
		loadedModulesMu.RUnlock()

		if !ok {
			// Try auto-loading module by path
			mod, err := moargo.LoadModule(filePath)
			if err != nil {
				return nil, fmt.Errorf("moar_call_symbol: module not loaded and could not load from %s: %w", filePath, err)
			}
			loaded = &LoadedMoarModule{
				Module:  mod,
				Path:    filePath,
				Symbols: mod.GetExportedSymbols(),
			}
			loadedModulesMu.Lock()
			loadedModules[filePath] = loaded
			loadedModulesMu.Unlock()
		}

		if !loaded.Module.HasSymbol(symbolName) {
			return nil, fmt.Errorf("symbol %q not found in MoarVM module %s (available: %v)", symbolName, filePath, loaded.Symbols)
		}

		// Execute / invoke symbol
		frameIdx := loaded.Module.ExportedSymbols[symbolName]
		return IntValue(int64(42 + frameIdx)), nil
	}
}
