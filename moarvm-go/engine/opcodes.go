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

	OpCheckArity uint16 = 140
	OpParamRpI   uint16 = 141
	OpParamRpN   uint16 = 142
	OpParamRpS   uint16 = 143
	OpParamRpO   uint16 = 144

	OpGetCode     uint16 = 159
	OpCaptureLex  uint16 = 161
	OpTakeClosure uint16 = 162

	OpCoerceIN uint16 = 119
	OpCoerceNI uint16 = 120
	OpCoerceIS uint16 = 121
	OpCoerceNS uint16 = 122
	OpCoerceSI uint16 = 123
	OpCoerceSN uint16 = 124
	OpCoerceIs uint16 = 452 // coerce_Is: obj → str
	OpCoerceIn uint16 = 451 // coerce_In: obj → num

	OpNull uint16 = 247

	OpDispatchV uint16 = 826
	OpDispatchI uint16 = 827
	OpDispatchN uint16 = 828
	OpDispatchS uint16 = 829
	OpDispatchO uint16 = 830

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
	OpBandI    uint16 = 76
	OpBorI     uint16 = 77
	OpBxorI    uint16 = 78
	OpBnotI    uint16 = 79
	OpBlshiftI uint16 = 80
	OpBrshiftI uint16 = 81
	OpPowI     uint16 = 82
	OpNotI     uint16 = 83

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

	// Official numbers from vendor/MoarVM/src/core/ops.h
	OpPrint uint16 = 494
	OpSay   uint16 = 495

	OpContinuationReset   uint16 = 551
	OpContinuationControl uint16 = 552
	OpContinuationInvoke  uint16 = 553

	OpSetElemsPos uint16 = 315
	OpShiftI      uint16 = 306
	OpShiftN      uint16 = 307
	OpShiftS      uint16 = 308
	OpShiftO      uint16 = 309
	OpUnshiftI    uint16 = 310
	OpUnshiftN    uint16 = 311
	OpUnshiftS    uint16 = 312
	OpUnshiftO    uint16 = 313
	OpSplice      uint16 = 314
	OpBootInt      uint16 = 339
	OpBootNum      uint16 = 340
	OpBootStr      uint16 = 341
	OpBootIntArray uint16 = 343
	OpBootNumArray uint16 = 344
	OpBootStrArray uint16 = 345
	OpStat    uint16 = 498
	OpSeekFH  uint16 = 477
	OpTellFH  uint16 = 497
	OpCwd     uint16 = 512
	OpChdir   uint16 = 501
	OpGetPID  uint16 = 533
	OpExit    uint16 = 510
	OpIsTrueS uint16 = 245
	OpIsNull  uint16 = 248
	OpGetCPS  uint16 = 214 // getcp_s: codepoint at index

	OpOpenFH   uint16 = 471
	OpCloseFH  uint16 = 472
	OpEofFH    uint16 = 482
	OpReadFHB  uint16 = 540
	OpWriteFHB uint16 = 541
	OpEncode   uint16 = 243
	OpDecode   uint16 = 244

	OpNativeCallBuild  uint16 = 564
	OpNativeCallInvoke uint16 = 565
	OpNativeCallGlobal uint16 = 617

	OpCopyF   uint16 = 460
	OpRenameF uint16 = 462
	OpDeleteF uint16 = 463
	OpExistsF uint16 = 465
	OpMkdir   uint16 = 466
	OpRmdir   uint16 = 467
)
