//go:build amd64 && !purego

#include "textflag.h"

// func hasAVX2Asm() bool
TEXT ·hasAVX2Asm(SB), NOSPLIT, $0-1
	MOVL $0, AX
	CPUID
	CMPL AX, $7
	JLT  no_avx2

	MOVL $1, AX
	MOVL $0, CX
	CPUID
	// ECX bit 27 = OSXSAVE, bit 28 = AVX -> 0x18000000
	ANDL $0x18000000, CX
	CMPL CX, $0x18000000
	JNE  no_avx2

	// XGETBV (ECX=0) -> EAX bits 1 (XMM) and 2 (YMM) must be set (0x6)
	MOVL $0, CX
	BYTE $0x0f; BYTE $0x01; BYTE $0xd0
	ANDL $0x6, AX
	CMPL AX, $0x6
	JNE  no_avx2

	// CPUID leaf 7, subleaf 0 -> EBX bit 5 = AVX2 (0x20)
	MOVL $7, AX
	MOVL $0, CX
	CPUID
	ANDL $0x20, BX
	CMPL BX, $0x20
	JNE  no_avx2

	MOVB $1, ret+0(FP)
	RET

no_avx2:
	MOVB $0, ret+0(FP)
	RET

// func avx2SumInt64(ptr *int64, n int) int64
TEXT ·avx2SumInt64(SB), NOSPLIT, $0-24
	MOVQ ptr+0(FP), SI
	MOVQ n+8(FP), CX
	XORQ AX, AX
	CMPQ CX, $0
	JLE  sum_i64_done

	VPXOR Y0, Y0, Y0
	VPXOR Y1, Y1, Y1
	VPXOR Y2, Y2, Y2
	VPXOR Y3, Y3, Y3

	CMPQ CX, $16
	JLT  sum_i64_fold4

sum_i64_loop16:
	VPADDQ 0(SI), Y0, Y0
	VPADDQ 32(SI), Y1, Y1
	VPADDQ 64(SI), Y2, Y2
	VPADDQ 96(SI), Y3, Y3
	ADDQ   $128, SI
	SUBQ   $16, CX
	CMPQ   CX, $16
	JGE    sum_i64_loop16

	VPADDQ Y1, Y0, Y0
	VPADDQ Y3, Y2, Y2
	VPADDQ Y2, Y0, Y0

sum_i64_fold4:
	CMPQ CX, $4
	JLT  sum_i64_reduce

sum_i64_loop4:
	VPADDQ 0(SI), Y0, Y0
	ADDQ   $32, SI
	SUBQ   $4, CX
	CMPQ   CX, $4
	JGE    sum_i64_loop4

sum_i64_reduce:
	VEXTRACTI128 $1, Y0, X1
	VPADDQ       X1, X0, X0
	VPSRLDQ      $8, X0, X1
	VPADDQ       X1, X0, X0
	VMOVQ        X0, AX
	VZEROUPPER

	CMPQ CX, $0
	JLE  sum_i64_done

sum_i64_tail:
	ADDQ 0(SI), AX
	ADDQ $8, SI
	DECQ CX
	JNZ  sum_i64_tail

sum_i64_done:
	MOVQ AX, ret+16(FP)
	RET

// func avx2SumFloat64(ptr *float64, n int) float64
TEXT ·avx2SumFloat64(SB), NOSPLIT, $0-24
	MOVQ   ptr+0(FP), SI
	MOVQ   n+8(FP), CX
	VXORPD X0, X0, X0
	CMPQ   CX, $0
	JLE    sum_f64_done

	VXORPD Y0, Y0, Y0
	VXORPD Y1, Y1, Y1
	VXORPD Y2, Y2, Y2
	VXORPD Y3, Y3, Y3

	CMPQ CX, $16
	JLT  sum_f64_fold4

sum_f64_loop16:
	VADDPD 0(SI), Y0, Y0
	VADDPD 32(SI), Y1, Y1
	VADDPD 64(SI), Y2, Y2
	VADDPD 96(SI), Y3, Y3
	ADDQ   $128, SI
	SUBQ   $16, CX
	CMPQ   CX, $16
	JGE    sum_f64_loop16

	VADDPD Y1, Y0, Y0
	VADDPD Y3, Y2, Y2
	VADDPD Y2, Y0, Y0

sum_f64_fold4:
	CMPQ CX, $4
	JLT  sum_f64_reduce

sum_f64_loop4:
	VADDPD 0(SI), Y0, Y0
	ADDQ   $32, SI
	SUBQ   $4, CX
	CMPQ   CX, $4
	JGE    sum_f64_loop4

sum_f64_reduce:
	VEXTRACTF128 $1, Y0, X1
	VADDPD       X1, X0, X0
	VUNPCKHPD    X0, X0, X1
	VADDSD       X1, X0, X0
	VZEROUPPER

	CMPQ CX, $0
	JLE  sum_f64_done

sum_f64_tail:
	ADDSD 0(SI), X0
	ADDQ  $8, SI
	DECQ  CX
	JNZ   sum_f64_tail

sum_f64_done:
	MOVSD X0, ret+16(FP)
	RET

// func avx2DotFloat64(a, b *float64, n int) float64
TEXT ·avx2DotFloat64(SB), NOSPLIT, $0-32
	MOVQ   a+0(FP), SI
	MOVQ   b+8(FP), DI
	MOVQ   n+16(FP), CX
	VXORPD X0, X0, X0
	CMPQ   CX, $0
	JLE    dot_f64_done

	VXORPD Y0, Y0, Y0
	VXORPD Y1, Y1, Y1

	CMPQ CX, $8
	JLT  dot_f64_fold4

dot_f64_loop8:
	VMOVUPD 0(SI), Y2
	VMULPD  0(DI), Y2, Y2
	VADDPD  Y2, Y0, Y0
	VMOVUPD 32(SI), Y3
	VMULPD  32(DI), Y3, Y3
	VADDPD  Y3, Y1, Y1
	ADDQ    $64, SI
	ADDQ    $64, DI
	SUBQ    $8, CX
	CMPQ    CX, $8
	JGE     dot_f64_loop8

	VADDPD Y1, Y0, Y0

dot_f64_fold4:
	CMPQ CX, $4
	JLT  dot_f64_reduce

dot_f64_loop4:
	VMOVUPD 0(SI), Y2
	VMULPD  0(DI), Y2, Y2
	VADDPD  Y2, Y0, Y0
	ADDQ    $32, SI
	ADDQ    $32, DI
	SUBQ    $4, CX
	CMPQ    CX, $4
	JGE     dot_f64_loop4

dot_f64_reduce:
	VEXTRACTF128 $1, Y0, X1
	VADDPD       X1, X0, X0
	VUNPCKHPD    X0, X0, X1
	VADDSD       X1, X0, X0
	VZEROUPPER

	CMPQ CX, $0
	JLE  dot_f64_done

dot_f64_tail:
	MOVSD 0(SI), X1
	MULSD 0(DI), X1
	ADDSD X1, X0
	ADDQ  $8, SI
	ADDQ  $8, DI
	DECQ  CX
	JNZ   dot_f64_tail

dot_f64_done:
	MOVSD X0, ret+24(FP)
	RET

// func avx2AddInt64(dst, a, b *int64, n int)
TEXT ·avx2AddInt64(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DX
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DI
	MOVQ n+24(FP), CX
	CMPQ CX, $4
	JLT  add_i64_tail_chk

add_i64_loop4:
	VMOVDQU 0(SI), Y0
	VPADDQ  0(DI), Y0, Y0
	VMOVDQU Y0, 0(DX)
	ADDQ    $32, SI
	ADDQ    $32, DI
	ADDQ    $32, DX
	SUBQ    $4, CX
	CMPQ    CX, $4
	JGE     add_i64_loop4

	VZEROUPPER

add_i64_tail_chk:
	CMPQ CX, $0
	JLE  add_i64_done

add_i64_tail:
	MOVQ 0(SI), AX
	ADDQ 0(DI), AX
	MOVQ AX, 0(DX)
	ADDQ $8, SI
	ADDQ $8, DI
	ADDQ $8, DX
	DECQ CX
	JNZ  add_i64_tail

add_i64_done:
	RET
