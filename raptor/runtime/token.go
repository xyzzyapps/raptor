package raptor

import "fmt"

// TokenType represents lexical token categories for Raku5.
type TokenType int

const (
	TokEOF TokenType = iota
	TokError

	// Literals
	TokInt
	TokFloat
	TokString

	// Identifiers & Variables
	TokIdent
	TokScalar // $foo
	TokArray  // @foo
	TokHash   // %foo
	TokSubRef // &foo

	// Keywords
	TokMy
	TokOur
	TokState
	TokPackage
	TokModule
	TokUnit
	TokSub
	TokMulti // multi
	TokReturn
	TokIf
	TokElsif
	TokElse
	TokUnless
	TokWhile
	TokUntil
	TokFor
	TokLoop
	TokUse
	TokSubset // subset

	// Operators - Arithmetic
	TokPlus
	TokMinus
	TokStar
	TokSlash
	TokPercent
	TokPower

	// Operators - Assignment
	TokAssign
	TokAddAssign
	TokSubAssign
	TokConcatAssign
	TokDefinedOrAssign // //=

	// Operators - Comparison
	TokEqual
	TokNotEqual
	TokLess
	TokLessEq
	TokGreater
	TokGreaterEq
	TokEqStr // eq
	TokNeStr // ne
	TokLtStr // lt
	TokGtStr // gt

	// Operators - Logic & String & Bitwise & Perl5/Raku
	TokAnd
	TokOr
	TokNot
	TokConcat // ~
	TokRepeat // x
	TokListRepeat // xx
	TokDefinedOr // //
	TokQuestionQuestion // ??
	TokExclamationExclamation // !!
	TokQuestion // ?
	TokRegexMatch // =~
	TokRegexNotMatch // !~
	TokBitAnd // +&
	TokBitOr // +|
	TokBitXor // +^
	TokBitShiftL // +<
	TokBitShiftR // +>
	TokDiv // div
	TokMod // mod
	TokDivisible // %%
	TokMin // min
	TokMax // max
	TokFileTest // -e, -f, -d, -s, -r, -w
	TokFatArrow    // =>
	TokArrow       // ->
	TokSmartMatch  // ~~
	TokDotDot      // ..
	TokCaretDotDot // ^..
	TokEllipsis    // ... (yada-yada / stub operator)
	TokElem        // ∈
	TokNotElem     // ∉
	TokIntersect   // ∩
	TokUnionOp     // ∪
	TokSo          // so (boolean truth operator)


	// Keywords - Control & OOP & FFI & Advice
	TokGiven
	TokWhen
	TokDefault
	TokEnum
	TokLast // last (break)
	TokNext // next (continue)
	TokStruct
	TokUnion
	TokWhere
	TokAssert
	TokBefore
	TokAfter
	TokAround
	TokReturns
	TokGoto // goto
	TokGrammar // grammar
	TokRule    // rule
	TokToken   // token
	TokRegex   // regex

	// Literals - Interpolated & Heredoc & Backtick
	TokInterpString // double-quoted string with interpolation markers
	TokHeredoc      // heredoc multiline string
	TokBacktick     // backtick raw command string
	TokInterpBacktick // backtick interpolated command string

	// Delimiters
	TokSemicolon
	TokComma
	TokDot
	TokLParen
	TokRParen
	TokLBrace
	TokRBrace
	TokLBracket
	TokRBracket
	TokAngleL // <
	TokAngleR // >
	TokColon  // :
	TokBackslash // \
)



// Token represents a single lexical token.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, %q, %d:%d)", t.Type, t.Literal, t.Line, t.Col)
}
