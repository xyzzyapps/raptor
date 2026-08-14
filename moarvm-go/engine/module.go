package moargo

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
)

// Module represents a compiled MoarVM dynamic module / library (.moarvm container)
// capable of exporting and resolving named symbols (MVMCode routines, frames, types).
type Module struct {
	Name            string
	HLL             string
	Emitter         *CompUnitEmitter
	ExportedSymbols map[string]uint32 // Symbol name -> Frame index
	Bytecode        []byte
}

// NewModule creates a new dynamic module with the given name and HLL identity.
func NewModule(name, hll string) *Module {
	cu := NewCompUnitEmitter(hll)
	return &Module{
		Name:            name,
		HLL:             hll,
		Emitter:         cu,
		ExportedSymbols: make(map[string]uint32),
	}
}

// DefineProc creates and registers an exported procedure / frame in the module.
func (m *Module) DefineProc(name string, numLocals int) (*FrameEmitter, uint32) {
	frame := m.Emitter.NewFrame(name, numLocals)
	frameIdx := uint32(len(m.Emitter.frames) - 1)
	m.ExportedSymbols[name] = frameIdx
	return frame, frameIdx
}

// ExportSymbol registers a frame as a public exported symbol in the module.
func (m *Module) ExportSymbol(name string, frameIndex uint32) {
	m.ExportedSymbols[name] = frameIndex
}

// GetExportedSymbols returns a list of all exported symbol names in the module.
func (m *Module) GetExportedSymbols() []string {
	var symbols []string
	for s := range m.ExportedSymbols {
		symbols = append(symbols, s)
	}
	return symbols
}

// HasSymbol checks if a named symbol is exported by this module.
func (m *Module) HasSymbol(name string) bool {
	_, ok := m.ExportedSymbols[name]
	return ok
}

// Build serializes the module into standard MoarVM CompUnit v7 bytecode with an embedded export manifest.
func (m *Module) Build() ([]byte, error) {
	baseBytes, err := m.Emitter.Emit()
	if err != nil {
		return nil, fmt.Errorf("module build failed: %w", err)
	}

	// Append export symbol table trailer for dynamic loader resolution
	var buf bytes.Buffer
	buf.Write(baseBytes)

	// Magic trailer for dynamic symbol resolution
	buf.WriteString("MOAR_DYNLIB_SYMS\x00")
	binary.Write(&buf, binary.LittleEndian, uint32(len(m.ExportedSymbols)))
	for sym, frameIdx := range m.ExportedSymbols {
		symBytes := []byte(sym)
		binary.Write(&buf, binary.LittleEndian, uint32(len(symBytes)))
		buf.Write(symBytes)
		binary.Write(&buf, binary.LittleEndian, frameIdx)
	}

	m.Bytecode = buf.Bytes()
	return m.Bytecode, nil
}

// Save writes the compiled dynamic module to a .moarvm file on disk.
func (m *Module) Save(filePath string) error {
	data, err := m.Build()
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// LoadModule reads and loads a compiled .moarvm dynamic module from disk.
func LoadModule(filePath string) (*Module, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed reading module %s: %w", filePath, err)
	}
	return LoadModuleBytes(data, filePath)
}

// LoadModuleBytes parses a compiled .moarvm dynamic module from memory.
func LoadModuleBytes(data []byte, name string) (*Module, error) {
	if len(data) < 96 {
		return nil, fmt.Errorf("invalid moarvm module: file too small (%d bytes)", len(data))
	}
	if !bytes.Equal(data[:4], []byte{'M', 'O', 'A', 'R'}) {
		return nil, fmt.Errorf("invalid moarvm module: invalid magic header")
	}

	mod := &Module{
		Name:            name,
		HLL:             "moar",
		ExportedSymbols: make(map[string]uint32),
		Bytecode:        data,
	}

	// Parse optional export symbol trailer if present
	trailerMarker := []byte("MOAR_DYNLIB_SYMS\x00")
	idx := bytes.LastIndex(data, trailerMarker)
	if idx != -1 {
		reader := bytes.NewReader(data[idx+len(trailerMarker):])
		var numSyms uint32
		if err := binary.Read(reader, binary.LittleEndian, &numSyms); err == nil {
			for i := uint32(0); i < numSyms; i++ {
				var nameLen uint32
				if err := binary.Read(reader, binary.LittleEndian, &nameLen); err != nil {
					break
				}
				nameBuf := make([]byte, nameLen)
				if _, err := reader.Read(nameBuf); err != nil {
					break
				}
				var frameIdx uint32
				if err := binary.Read(reader, binary.LittleEndian, &frameIdx); err != nil {
					break
				}
				mod.ExportedSymbols[string(nameBuf)] = frameIdx
			}
		}
	}

	return mod, nil
}

// Execute runs the module's mainline entrypoint on a MoarVM engine instance.
func (m *Module) Execute(ctx context.Context, vm Engine) error {
	if len(m.Bytecode) == 0 {
		if _, err := m.Build(); err != nil {
			return err
		}
	}
	return vm.RunBytecode(ctx, m.Bytecode)
}
