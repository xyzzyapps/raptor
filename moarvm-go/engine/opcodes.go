package moargo

// MoarVM register kinds
const (
	RegInt8   uint16 = 1
	RegInt16  uint16 = 2
	RegInt32  uint16 = 3
	RegInt64  uint16 = 4
	RegNum32  uint16 = 5
	RegNum64  uint16 = 6
	RegStr    uint16 = 7
	RegObj    uint16 = 8
	RegUint8  uint16 = 17
	RegUint16 uint16 = 18
	RegUint32 uint16 = 19
	RegUint64 uint16 = 20
)

// MoarVM Opcode Constants (from src/core/oplist & ops.h)
const (
	OpNoOp    uint16 = 0
	OpConstI8 uint16 = 1
	OpConstI16 uint16 = 2
	OpConstI32 uint16 = 3
	OpConstI64 uint16 = 4
	OpConstN32 uint16 = 5
	OpConstN64 uint16 = 6
	OpConstS   uint16 = 7
	OpSet      uint16 = 8

	OpGoto    uint16 = 23
	OpIfI     uint16 = 24
	OpUnlessI uint16 = 25
	OpIfN     uint16 = 26
	OpUnlessN uint16 = 27
	OpIfS     uint16 = 28
	OpUnlessS uint16 = 29

	OpGetLex  uint16 = 35
	OpBindLex uint16 = 36

	OpReturnI uint16 = 51
	OpReturnN uint16 = 52
	OpReturnS uint16 = 53
	OpReturnO uint16 = 54
	OpReturn  uint16 = 55

	OpEqI  uint16 = 56
	OpNeI  uint16 = 57
	OpLtI  uint16 = 58
	OpLeI  uint16 = 59
	OpGtI  uint16 = 60
	OpGeI  uint16 = 61
	OpCmpI uint16 = 62
	OpAddI uint16 = 63
	OpSubI uint16 = 64
	OpMulI uint16 = 65
	OpDivI uint16 = 66
	OpModI uint16 = 68
	OpNegI uint16 = 70
	OpAbsI uint16 = 71
	OpIncI uint16 = 72
	OpDecI uint16 = 74
	OpPowI uint16 = 82
	OpNotI uint16 = 83

	OpEqN  uint16 = 86
	OpNeN  uint16 = 87
	OpLtN  uint16 = 88
	OpLeN  uint16 = 89
	OpGtN  uint16 = 90
	OpGeN  uint16 = 91
	OpAddN uint16 = 93
	OpSubN uint16 = 94
	OpMulN uint16 = 95
	OpDivN uint16 = 96

	OpEqS     uint16 = 198
	OpNeS     uint16 = 199
	OpGtS     uint16 = 200
	OpGeS     uint16 = 201
	OpLtS     uint16 = 202
	OpLeS     uint16 = 203
	OpCmpS    uint16 = 204
	OpConcatS uint16 = 208
	OpRepeatS uint16 = 209
	OpSubstrS uint16 = 210
	OpIndexS  uint16 = 211
	OpCodesS  uint16 = 213
	OpUC      uint16 = 216
	OpLC      uint16 = 217
	OpSplit   uint16 = 219
	OpJoin    uint16 = 220
	OpChars   uint16 = 228

	OpPrepArgs uint16 = 40
	OpArgI     uint16 = 41
	OpArgN     uint16 = 42
	OpArgS     uint16 = 43
	OpArgO     uint16 = 44
	OpInvoke   uint16 = 49

	OpAtPosI   uint16 = 290
	OpAtPosN   uint16 = 291
	OpAtPosS   uint16 = 292
	OpAtPosO   uint16 = 293
	OpBindPosI uint16 = 294
	OpBindPosN uint16 = 295
	OpBindPosS uint16 = 296
	OpBindPosO uint16 = 297

	OpSay      uint16 = 250
	OpPrint    uint16 = 251
)
