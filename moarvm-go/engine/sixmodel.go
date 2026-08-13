package moargo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// 6Model representation names
const (
	REPR_P6opaque = "P6opaque"
	REPR_MVMHash  = "MVMHash"
	REPR_VMArray  = "VMArray"
	REPR_KnowHOW  = "KnowHOW"
)

// 6Model Opcode Constants (mapped from MoarVM src/core/ops.h)
const (
	OpCreate       uint16 = 254
	OpClone        uint16 = 255
	OpIsConcrete   uint16 = 256
	OpRebless      uint16 = 257
	OpIsType       uint16 = 258
	OpGetHOW       uint16 = 260
	OpGetWHAT      uint16 = 261
	OpGetWHO       uint16 = 262
	OpSetWHO       uint16 = 263
	OpReprName     uint16 = 264
	OpGetWHERE     uint16 = 265
	OpEqAddr       uint16 = 266
	OpBindAttrI    uint16 = 267
	OpBindAttrN    uint16 = 268
	OpBindAttrS    uint16 = 269
	OpBindAttrO    uint16 = 270
	OpGetAttrI     uint16 = 275
	OpGetAttrN     uint16 = 276
	OpGetAttrS     uint16 = 277
	OpGetAttrO     uint16 = 278
	OpBoxI         uint16 = 284
	OpBoxN         uint16 = 285
	OpBoxS         uint16 = 286
	OpUnboxI       uint16 = 287
	OpUnboxN       uint16 = 288
	OpUnboxS       uint16 = 289
	OpPushI        uint16 = 298
	OpPushN        uint16 = 299
	OpPushS        uint16 = 300
	OpPushO        uint16 = 301
	OpPopI         uint16 = 302
	OpPopN         uint16 = 303
	OpPopS         uint16 = 304
	OpPopO         uint16 = 305
	OpAtKeyI       uint16 = 317
	OpAtKeyN       uint16 = 318
	OpAtKeyS       uint16 = 319
	OpAtKeyO       uint16 = 320
	OpBindKeyI     uint16 = 321
	OpBindKeyN     uint16 = 322
	OpBindKeyS     uint16 = 323
	OpBindKeyO     uint16 = 324
	OpExistsKey    uint16 = 325
	OpDeleteKey    uint16 = 326
	OpElems        uint16 = 327
	OpKnowHOW      uint16 = 328
	OpKnowHOWAttr  uint16 = 329
	OpNewType      uint16 = 330
	OpComposeType  uint16 = 331
	OpBootArray    uint16 = 342
	OpBootHash     uint16 = 346
	OpIsInt        uint16 = 347
	OpIsNum        uint16 = 348
	OpIsStr        uint16 = 349
	OpIsList       uint16 = 350
	OpIsHash       uint16 = 351
)

// SixModelClass describes a class metamodel for code generation.
type SixModelClass struct {
	Name       string
	Repr       string
	Attributes []SixModelAttribute
	Methods    map[string]*FrameEmitter
}

// SixModelAttribute describes a typed attribute on a 6Model class.
type SixModelAttribute struct {
	Name string
	Type uint16 // e.g. RegInt64, RegStr, RegObj
}

// NewSixModelClass creates a new 6Model class definition.
func NewSixModelClass(name string, repr string) *SixModelClass {
	if repr == "" {
		repr = REPR_P6opaque
	}
	return &SixModelClass{
		Name:    name,
		Repr:    repr,
		Methods: make(map[string]*FrameEmitter),
	}
}

func (c *SixModelClass) AddAttribute(name string, regType uint16) {
	c.Attributes = append(c.Attributes, SixModelAttribute{
		Name: name,
		Type: regType,
	})
}

func (c *SixModelClass) AddMethod(name string, fe *FrameEmitter) {
	c.Methods[name] = fe
}

// STable represents a 6Model shared table (type descriptor and method cache).
type STable struct {
	HOWName  string
	WHATName string
	WHOName  string
	ReprName string
	Methods  map[string]uint32 // method name -> frame index
}

// NewSTable creates a new STable definition.
func NewSTable(how, what, who, repr string) *STable {
	return &STable{
		HOWName:  how,
		WHATName: what,
		WHOName:  who,
		ReprName: repr,
		Methods:  make(map[string]uint32),
	}
}

// Repossession represents an object repossessed from another Serialization Context.
type Repossession struct {
	ObjIndex  uint32
	OrigSC    string
	OrigIndex uint32
}

// SerializationContext (SC) represents a full 6Model serialization context for compiling and loading class trees.
type SerializationContext struct {
	Handle        string
	Description   string
	Dependencies  []string
	RootSTables   []*STable
	RootObjects   [][]byte
	Repossessions []Repossession
}

// NewSerializationContext creates a new initialized SerializationContext.
func NewSerializationContext(handle, desc string) *SerializationContext {
	return &SerializationContext{
		Handle:      handle,
		Description: desc,
	}
}

// AddSTable adds an S-Table to the SC and returns its index.
func (sc *SerializationContext) AddSTable(st *STable) uint32 {
	idx := uint32(len(sc.RootSTables))
	sc.RootSTables = append(sc.RootSTables, st)
	return idx
}

// AddObject adds serialized object bytes to the SC and returns its root object index.
func (sc *SerializationContext) AddObject(objData []byte) uint32 {
	idx := uint32(len(sc.RootObjects))
	sc.RootObjects = append(sc.RootObjects, objData)
	return idx
}

// AddDependency adds a dependent SC handle name.
func (sc *SerializationContext) AddDependency(depHandle string) {
	sc.Dependencies = append(sc.Dependencies, depHandle)
}

// Serialize encodes the SerializationContext into a binary buffer matching CompUnit v7 format.
func (sc *SerializationContext) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Magic SC header: "MVMSC\x07" (7 bytes)
	buf.WriteString("MVMSC\x07")

	// Write Handle & Description
	if err := writeLengthPrefixedString(buf, sc.Handle); err != nil {
		return nil, err
	}
	if err := writeLengthPrefixedString(buf, sc.Description); err != nil {
		return nil, err
	}

	// Dependencies
	binary.Write(buf, binary.LittleEndian, uint32(len(sc.Dependencies)))
	for _, dep := range sc.Dependencies {
		if err := writeLengthPrefixedString(buf, dep); err != nil {
			return nil, err
		}
	}

	// STables count
	binary.Write(buf, binary.LittleEndian, uint32(len(sc.RootSTables)))
	for _, st := range sc.RootSTables {
		_ = writeLengthPrefixedString(buf, st.HOWName)
		_ = writeLengthPrefixedString(buf, st.WHATName)
		_ = writeLengthPrefixedString(buf, st.WHOName)
		_ = writeLengthPrefixedString(buf, st.ReprName)
		binary.Write(buf, binary.LittleEndian, uint32(len(st.Methods)))
		for mName, frameIdx := range st.Methods {
			_ = writeLengthPrefixedString(buf, mName)
			binary.Write(buf, binary.LittleEndian, frameIdx)
		}
	}

	// Root objects count & payload
	binary.Write(buf, binary.LittleEndian, uint32(len(sc.RootObjects)))
	for _, obj := range sc.RootObjects {
		binary.Write(buf, binary.LittleEndian, uint32(len(obj)))
		buf.Write(obj)
	}

	// Repossessions count
	binary.Write(buf, binary.LittleEndian, uint32(len(sc.Repossessions)))
	for _, repo := range sc.Repossessions {
		binary.Write(buf, binary.LittleEndian, repo.ObjIndex)
		_ = writeLengthPrefixedString(buf, repo.OrigSC)
		binary.Write(buf, binary.LittleEndian, repo.OrigIndex)
	}

	return buf.Bytes(), nil
}

// Deserialize unpacks a binary SerializationContext buffer into a *SerializationContext.
func DeserializeSerializationContext(data []byte) (*SerializationContext, error) {
	r := bytes.NewReader(data)

	header := make([]byte, 6)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("failed to read SC header: %w", err)
	}
	if string(header) != "MVMSC\x07" {
		return nil, fmt.Errorf("invalid SC magic header %q", string(header))
	}


	handle, err := readLengthPrefixedString(r)
	if err != nil {
		return nil, err
	}
	desc, err := readLengthPrefixedString(r)
	if err != nil {
		return nil, err
	}

	sc := NewSerializationContext(handle, desc)

	var numDeps uint32
	if err := binary.Read(r, binary.LittleEndian, &numDeps); err != nil {
		return nil, err
	}
	for i := uint32(0); i < numDeps; i++ {
		dep, err := readLengthPrefixedString(r)
		if err != nil {
			return nil, err
		}
		sc.AddDependency(dep)
	}

	var numSTables uint32
	if err := binary.Read(r, binary.LittleEndian, &numSTables); err != nil {
		return nil, err
	}
	for i := uint32(0); i < numSTables; i++ {
		how, _ := readLengthPrefixedString(r)
		what, _ := readLengthPrefixedString(r)
		who, _ := readLengthPrefixedString(r)
		repr, _ := readLengthPrefixedString(r)
		st := NewSTable(how, what, who, repr)

		var numMethods uint32
		_ = binary.Read(r, binary.LittleEndian, &numMethods)
		for j := uint32(0); j < numMethods; j++ {
			mName, _ := readLengthPrefixedString(r)
			var fIdx uint32
			_ = binary.Read(r, binary.LittleEndian, &fIdx)
			st.Methods[mName] = fIdx
		}
		sc.AddSTable(st)
	}

	var numObjs uint32
	if err := binary.Read(r, binary.LittleEndian, &numObjs); err != nil {
		return nil, err
	}
	for i := uint32(0); i < numObjs; i++ {
		var objLen uint32
		if err := binary.Read(r, binary.LittleEndian, &objLen); err != nil {
			return nil, err
		}
		objBytes := make([]byte, objLen)
		if _, err := io.ReadFull(r, objBytes); err != nil {
			return nil, err
		}
		sc.AddObject(objBytes)
	}

	var numRepos uint32
	if err := binary.Read(r, binary.LittleEndian, &numRepos); err != nil {
		return nil, err
	}
	for i := uint32(0); i < numRepos; i++ {
		var repo Repossession
		_ = binary.Read(r, binary.LittleEndian, &repo.ObjIndex)
		origSC, _ := readLengthPrefixedString(r)
		repo.OrigSC = origSC
		_ = binary.Read(r, binary.LittleEndian, &repo.OrigIndex)
		sc.Repossessions = append(sc.Repossessions, repo)
	}

	return sc, nil
}

func writeLengthPrefixedString(w io.Writer, s string) error {
	b := []byte(s)
	if err := binary.Write(w, binary.LittleEndian, uint32(len(b))); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func readLengthPrefixedString(r io.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
