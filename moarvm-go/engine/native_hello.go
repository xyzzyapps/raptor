package moargo

// EmitSayString builds a CompUnit whose mainline does `say $message; return`.
func EmitSayString(message string) ([]byte, error) {
	cu := NewCompUnitEmitter("tcl")
	f := cu.NewFrame("main", 2)
	f.SetLocalType(0, RegStr)
	f.EmitOp(OpConstS)
	f.EmitReg(0)
	f.EmitString(message)
	f.EmitOp(OpSay)
	f.EmitReg(0)
	f.EmitOp(OpReturn)
	return cu.Emit()
}
