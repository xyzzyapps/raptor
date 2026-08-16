package moargo

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// CompUnitEmitter builds a valid MoarVM version 7 Compilation Unit (.moarvm binary).
// Callsite flags (MVM_CALLSITE_ARG_*).
const (
	CallArgObj uint8 = 1
	CallArgInt uint8 = 2
	CallArgNum uint8 = 4
	CallArgStr uint8 = 8
)

type CompUnitEmitter struct {
	HLLName    string
	strings    []string
	stringMap  map[string]uint32
	frames     []*FrameEmitter
	callsites  [][]uint8
}

// NewCompUnitEmitter creates a new compilation unit builder.
func NewCompUnitEmitter(hllName string) *CompUnitEmitter {
	cu := &CompUnitEmitter{
		HLLName:   hllName,
		stringMap: make(map[string]uint32),
	}
	// Index 0 is HLL name
	cu.AddString(hllName)
	return cu
}

// AddString adds a string to the string heap and returns its 0-based index.
func (cu *CompUnitEmitter) AddString(s string) uint32 {
	if idx, ok := cu.stringMap[s]; ok {
		return idx
	}
	idx := uint32(len(cu.strings))
	cu.strings = append(cu.strings, s)
	cu.stringMap[s] = idx
	return idx
}

// FrameEmitter builds bytecode for an individual static frame.
type lexicalSlot struct {
	Name string
	Type uint16
}

type FrameEmitter struct {
	cu         *CompUnitEmitter
	Index      int
	Name       string
	CUUID      string
	NumLocals  uint32
	LocalTypes []uint16
	Lexicals   []lexicalSlot
	Outer      int // frame index; -1 means self (no outer)
	Bytecode   *bytes.Buffer
}

// NumFrames returns how many static frames are in the unit.
func (cu *CompUnitEmitter) NumFrames() int {
	return len(cu.frames)
}

// FrameAt returns the static frame at index i.
func (cu *CompUnitEmitter) FrameAt(i int) *FrameEmitter {
	return cu.frames[i]
}

// AddCallsite records a positional callsite and returns its index.
func (cu *CompUnitEmitter) AddCallsite(flags []uint8) uint16 {
	for i, f := range cu.callsites {
		if len(f) != len(flags) {
			continue
		}
		ok := true
		for j := range f {
			if f[j] != flags[j] {
				ok = false
				break
			}
		}
		if ok {
			return uint16(i)
		}
	}
	cu.callsites = append(cu.callsites, append([]uint8(nil), flags...))
	return uint16(len(cu.callsites) - 1)
}

// EmitGetCode writes getcode $dst, frameIndex.
func (fe *FrameEmitter) EmitGetCode(dst uint16, frameIndex uint16) {
	fe.EmitOp(OpGetCode)
	fe.EmitReg(dst)
	fe.EmitReg(frameIndex)
}

// EmitDispatchS writes dispatch_s $dst, dispatcherName, callsite, args...
func (fe *FrameEmitter) EmitDispatchS(dst uint16, dispatcher string, cs uint16, args ...uint16) {
	fe.EmitOp(OpDispatchS)
	fe.EmitReg(dst)
	fe.EmitString(dispatcher)
	fe.EmitReg(cs)
	for _, a := range args {
		fe.EmitReg(a)
	}
}

// EmitDispatchI writes dispatch_i $dst, dispatcherName, callsite, args...
func (fe *FrameEmitter) EmitDispatchI(dst uint16, dispatcher string, cs uint16, args ...uint16) {
	fe.EmitOp(OpDispatchI)
	fe.EmitReg(dst)
	fe.EmitString(dispatcher)
	fe.EmitReg(cs)
	for _, a := range args {
		fe.EmitReg(a)
	}
}

// NewFrame creates a new static frame in the compilation unit.
func (cu *CompUnitEmitter) NewFrame(name string, numLocals int) *FrameEmitter {
	cu.AddString(name)
	cu.AddString("cuuid_" + name)
	idx := len(cu.frames)
	fe := &FrameEmitter{
		cu:         cu,
		Index:      idx,
		Name:       name,
		CUUID:      "cuuid_" + name,
		NumLocals:  uint32(numLocals),
		LocalTypes: make([]uint16, numLocals),
		Outer:      idx,
		Bytecode:   new(bytes.Buffer),
	}
	for i := range fe.LocalTypes {
		fe.LocalTypes[i] = RegInt64
	}
	cu.frames = append(cu.frames, fe)
	return fe
}

func (fe *FrameEmitter) LexicalCount() int { return len(fe.Lexicals) }

func (fe *FrameEmitter) AddLexical(name string, typ uint16) uint16 {
	fe.cu.AddString(name)
	fe.Lexicals = append(fe.Lexicals, lexicalSlot{Name: name, Type: typ})
	return uint16(len(fe.Lexicals) - 1)
}

func (fe *FrameEmitter) SetOuter(frameIndex int) {
	fe.Outer = frameIndex
}

func (fe *FrameEmitter) SetLocalType(idx int, regKind uint16) {
	if idx >= 0 && idx < len(fe.LocalTypes) {
		fe.LocalTypes[idx] = regKind
	}
}

func (fe *FrameEmitter) EmitOp(op uint16) {
	binary.Write(fe.Bytecode, binary.LittleEndian, op)
}

func (fe *FrameEmitter) EmitReg(reg uint16) {
	binary.Write(fe.Bytecode, binary.LittleEndian, reg)
}

func (fe *FrameEmitter) EmitInt8(val int8) {
	binary.Write(fe.Bytecode, binary.LittleEndian, val)
}

func (fe *FrameEmitter) EmitInt16(val int16) {
	binary.Write(fe.Bytecode, binary.LittleEndian, val)
}

func (fe *FrameEmitter) EmitInt32(val int32) {
	binary.Write(fe.Bytecode, binary.LittleEndian, val)
}

func (fe *FrameEmitter) EmitInt64(val int64) {
	binary.Write(fe.Bytecode, binary.LittleEndian, val)
}

func (fe *FrameEmitter) EmitString(s string) {
	idx := fe.cu.AddString(s)
	binary.Write(fe.Bytecode, binary.LittleEndian, idx)
}

func (fe *FrameEmitter) CurrentOffset() int32 {
	return int32(fe.Bytecode.Len())
}

// Emit compiles all segments into a binary .moarvm file.
func (cu *CompUnitEmitter) Emit() ([]byte, error) {
	if len(cu.frames) == 0 {
		return nil, fmt.Errorf("compilation unit must have at least one frame")
	}

	// Register remaining frame/lexical names before packing the string heap.
	for _, f := range cu.frames {
		cu.AddString(f.CUUID)
		cu.AddString(f.Name)
		for _, lx := range f.Lexicals {
			cu.AddString(lx.Name)
		}
	}

	// 1. Pack string heap segment
	var stringSeg bytes.Buffer
	for _, s := range cu.strings {
		sBytes := []byte(s)
		sLen := uint32(len(sBytes))
		ss := (sLen << 1) | 1 // UTF-8 flag
		binary.Write(&stringSeg, binary.LittleEndian, ss)
		stringSeg.Write(sBytes)
		// Pad to 4-byte boundary
		pad := sLen & 3
		if pad != 0 {
			stringSeg.Write(make([]byte, 4-pad))
		}
	}

	// 2. Pack bytecode segment from all frames
	var bytecodeSeg bytes.Buffer
	frameBCOffsets := make([]uint32, len(cu.frames))
	frameBCSizes := make([]uint32, len(cu.frames))
	for i, f := range cu.frames {
		frameBCOffsets[i] = uint32(bytecodeSeg.Len())
		b := f.Bytecode.Bytes()
		frameBCSizes[i] = uint32(len(b))
		bytecodeSeg.Write(b)
	}

	// 3. Pack frames segment
	var frameSeg bytes.Buffer
	for i, f := range cu.frames {
		cuuidIdx := cu.AddString(f.CUUID)
		nameIdx := cu.AddString(f.Name)

		// Frame header (FRAME_HEADER_SIZE = 54 bytes)
		binary.Write(&frameSeg, binary.LittleEndian, frameBCOffsets[i]) // 0..3 bytecode_pos
		binary.Write(&frameSeg, binary.LittleEndian, frameBCSizes[i])   // 4..7 bytecode_size
		outer := f.Outer
		if outer < 0 {
			outer = i
		}
		binary.Write(&frameSeg, binary.LittleEndian, f.NumLocals)                 // 8..11 num_locals
		binary.Write(&frameSeg, binary.LittleEndian, uint32(len(f.Lexicals)))     // 12..15 num_lexicals
		binary.Write(&frameSeg, binary.LittleEndian, cuuidIdx)                    // 16..19 cuuid string index
		binary.Write(&frameSeg, binary.LittleEndian, nameIdx)                     // 20..23 name string index
		binary.Write(&frameSeg, binary.LittleEndian, uint16(outer))               // 24..25 outer_fixup
		binary.Write(&frameSeg, binary.LittleEndian, uint32(0))         // 26..29 annot_offset
		binary.Write(&frameSeg, binary.LittleEndian, uint32(0))         // 30..33 num_annotations
		binary.Write(&frameSeg, binary.LittleEndian, uint32(0))         // 34..37 num_handlers
		binary.Write(&frameSeg, binary.LittleEndian, uint16(0))         // 38..39 flags
		binary.Write(&frameSeg, binary.LittleEndian, uint16(0))         // 40..41 slvs
		binary.Write(&frameSeg, binary.LittleEndian, uint32(0))         // 42..45 code_obj_sc_dep_idx
		binary.Write(&frameSeg, binary.LittleEndian, uint32(0))         // 46..49 code_obj_sc_idx
		binary.Write(&frameSeg, binary.LittleEndian, uint32(0))         // 50..53 num_local_debug_names

		// Local register types: 2 * num_locals bytes
		for _, lt := range f.LocalTypes {
			binary.Write(&frameSeg, binary.LittleEndian, lt)
		}
		for _, lx := range f.Lexicals {
			binary.Write(&frameSeg, binary.LittleEndian, lx.Type)
			binary.Write(&frameSeg, binary.LittleEndian, cu.AddString(lx.Name))
		}
	}

	// 4. Empty auxiliary segments
	var scSeg bytes.Buffer
	var extopSeg bytes.Buffer
	var callsiteSeg bytes.Buffer
	for _, flags := range cu.callsites {
		binary.Write(&callsiteSeg, binary.LittleEndian, uint16(len(flags)))
		callsiteSeg.Write(flags)
		if len(flags)%2 != 0 {
			callsiteSeg.WriteByte(0)
		}
	}
	var scDataSeg bytes.Buffer
	var annotationSeg bytes.Buffer

	// Exact header size is 96 bytes
	const headerSize = 96
	scOffset := uint32(headerSize)
	scCount := uint32(0)

	extopOffset := scOffset + uint32(scSeg.Len())
	extopCount := uint32(0)

	frameOffset := extopOffset + uint32(extopSeg.Len())
	frameCount := uint32(len(cu.frames))

	callsiteOffset := frameOffset + uint32(frameSeg.Len())
	callsiteCount := uint32(len(cu.callsites))

	stringOffset := callsiteOffset + uint32(callsiteSeg.Len())
	stringCount := uint32(len(cu.strings))

	scdataOffset := stringOffset + uint32(stringSeg.Len())
	scdataSize := uint32(scDataSeg.Len())

	bytecodeOffset := scdataOffset + scdataSize
	bytecodeSize := uint32(bytecodeSeg.Len())

	annotationOffset := bytecodeOffset + bytecodeSize
	annotationSize := uint32(annotationSeg.Len())

	hllNameIdx := uint32(0)
	mainlineFrame := uint32(1) // 1-based index (frame 0 + 1)
	mainFrame := uint32(1)
	loadFrame := uint32(0)
	deserializeFrame := uint32(0)

	// Write 96-byte header
	var out bytes.Buffer
	out.WriteString("MOARVM\r\n")                               // 0..7 Magic
	binary.Write(&out, binary.LittleEndian, uint32(7))          // 8..11 Version = 7
	binary.Write(&out, binary.LittleEndian, scOffset)           // 12..15
	binary.Write(&out, binary.LittleEndian, scCount)            // 16..19
	binary.Write(&out, binary.LittleEndian, extopOffset)        // 20..23
	binary.Write(&out, binary.LittleEndian, extopCount)         // 24..27
	binary.Write(&out, binary.LittleEndian, frameOffset)        // 28..31
	binary.Write(&out, binary.LittleEndian, frameCount)         // 32..35
	binary.Write(&out, binary.LittleEndian, callsiteOffset)     // 36..39
	binary.Write(&out, binary.LittleEndian, callsiteCount)      // 40..43
	binary.Write(&out, binary.LittleEndian, stringOffset)       // 44..47
	binary.Write(&out, binary.LittleEndian, stringCount)        // 48..51
	binary.Write(&out, binary.LittleEndian, scdataOffset)       // 52..55
	binary.Write(&out, binary.LittleEndian, scdataSize)         // 56..59
	binary.Write(&out, binary.LittleEndian, bytecodeOffset)     // 60..63
	binary.Write(&out, binary.LittleEndian, bytecodeSize)       // 64..67
	binary.Write(&out, binary.LittleEndian, annotationOffset)   // 68..71
	binary.Write(&out, binary.LittleEndian, annotationSize)     // 72..75
	binary.Write(&out, binary.LittleEndian, hllNameIdx)         // 76..79
	binary.Write(&out, binary.LittleEndian, mainlineFrame)      // 80..83
	binary.Write(&out, binary.LittleEndian, mainFrame)          // 84..87
	binary.Write(&out, binary.LittleEndian, loadFrame)          // 88..91
	binary.Write(&out, binary.LittleEndian, deserializeFrame)   // 92..95

	// Write segments in order
	out.Write(scSeg.Bytes())
	out.Write(extopSeg.Bytes())
	out.Write(frameSeg.Bytes())
	out.Write(callsiteSeg.Bytes())
	out.Write(stringSeg.Bytes())
	out.Write(scDataSeg.Bytes())
	out.Write(bytecodeSeg.Bytes())
	out.Write(annotationSeg.Bytes())

	return out.Bytes(), nil
}
