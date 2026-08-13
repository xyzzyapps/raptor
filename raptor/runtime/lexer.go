package raptor

import (
	"fmt"
	"strings"
	"unicode"
)

// Lexer scans source code into a token stream for Raku5.
type Lexer struct {
	src  []rune
	pos  int
	line int
	col  int
}

// NewLexer creates a new Lexer instance.
func NewLexer(input string) *Lexer {
	return &Lexer{
		src:  []rune(input),
		pos:  0,
		line: 1,
		col:  1,
	}
}

// NextToken returns the next Token from the input stream.
func (l *Lexer) NextToken() Token {
	l.skipWhitespaceAndComments()

	if l.pos >= len(l.src) {
		return Token{Type: TokEOF, Line: l.line, Col: l.col}
	}

	ch := l.src[l.pos]
	curLine := l.line
	curCol := l.col

	// Sigils: $, @, %
	if ch == '$' || ch == '@' || ch == '%' {
		if l.pos+1 < len(l.src) && (unicode.IsLetter(l.src[l.pos+1]) || l.src[l.pos+1] == '_' || l.src[l.pos+1] == '*' || l.src[l.pos+1] == '!') {
			return l.scanVariable()
		}
	}

	// Numbers
	if unicode.IsDigit(ch) {
		return l.scanNumber()
	}

	// Strings
	if ch == '"' || ch == '\'' {
		return l.scanString(ch)
	}

	// Identifiers & Keywords & Types
	if unicode.IsLetter(ch) || ch == '_' {
		return l.scanIdentOrKeyword()
	}

	// Three-character operators
	if l.pos+2 < len(l.src) {
		triplet := string(l.src[l.pos : l.pos+3])
		if triplet == "//=" {
			l.advance(); l.advance(); l.advance()
			return Token{Type: TokDefinedOrAssign, Literal: "//=", Line: curLine, Col: curCol}
		}
	}

	// File test operators: -e, -f, -d, -s, -r, -w
	if ch == '-' && l.pos+1 < len(l.src) {
		nextCh := l.src[l.pos+1]
		if nextCh == 'e' || nextCh == 'f' || nextCh == 'd' || nextCh == 's' || nextCh == 'r' || nextCh == 'w' {
			if l.pos+2 >= len(l.src) || l.src[l.pos+2] == ' ' || l.src[l.pos+2] == '\t' || l.src[l.pos+2] == '$' || l.src[l.pos+2] == '"' || l.src[l.pos+2] == '\'' || l.src[l.pos+2] == '(' {
				l.advance(); l.advance()
				return Token{Type: TokFileTest, Literal: "-" + string(nextCh), Line: curLine, Col: curCol}
			}
		}
	}

	// Two-character operators
	if l.pos+1 < len(l.src) {
		pair := string(l.src[l.pos : l.pos+2])
		switch pair {
		case "==":
			l.advance(); l.advance()
			return Token{Type: TokEqual, Literal: "==", Line: curLine, Col: curCol}
		case "!=":
			l.advance(); l.advance()
			return Token{Type: TokNotEqual, Literal: "!=", Line: curLine, Col: curCol}
		case "<=":
			l.advance(); l.advance()
			return Token{Type: TokLessEq, Literal: "<=", Line: curLine, Col: curCol}
		case ">=":
			l.advance(); l.advance()
			return Token{Type: TokGreaterEq, Literal: ">=", Line: curLine, Col: curCol}
		case "+=":
			l.advance(); l.advance()
			return Token{Type: TokAddAssign, Literal: "+=", Line: curLine, Col: curCol}
		case "-=":
			l.advance(); l.advance()
			return Token{Type: TokSubAssign, Literal: "-=", Line: curLine, Col: curCol}
		case "~=":
			l.advance(); l.advance()
			return Token{Type: TokConcatAssign, Literal: "~=", Line: curLine, Col: curCol}
		case "=>":
			l.advance(); l.advance()
			return Token{Type: TokFatArrow, Literal: "=>", Line: curLine, Col: curCol}
		case "->":
			l.advance(); l.advance()
			return Token{Type: TokArrow, Literal: "->", Line: curLine, Col: curCol}
		case "**":
			l.advance(); l.advance()
			return Token{Type: TokPower, Literal: "**", Line: curLine, Col: curCol}
		case "&&":
			l.advance(); l.advance()
			return Token{Type: TokAnd, Literal: "&&", Line: curLine, Col: curCol}
		case "~~":
			l.advance(); l.advance()
			return Token{Type: TokSmartMatch, Literal: "~~", Line: curLine, Col: curCol}
		case "=~":
			l.advance(); l.advance()
			return Token{Type: TokRegexMatch, Literal: "=~", Line: curLine, Col: curCol}
		case "!~":
			l.advance(); l.advance()
			return Token{Type: TokRegexNotMatch, Literal: "!~", Line: curLine, Col: curCol}
		case "//":
			l.advance(); l.advance()
			return Token{Type: TokDefinedOr, Literal: "//", Line: curLine, Col: curCol}
		case "??":
			l.advance(); l.advance()
			return Token{Type: TokQuestionQuestion, Literal: "??", Line: curLine, Col: curCol}
		case "!!":
			l.advance(); l.advance()
			return Token{Type: TokExclamationExclamation, Literal: "!!", Line: curLine, Col: curCol}
		case "+&":
			l.advance(); l.advance()
			return Token{Type: TokBitAnd, Literal: "+&", Line: curLine, Col: curCol}
		case "+|":
			l.advance(); l.advance()
			return Token{Type: TokBitOr, Literal: "+|", Line: curLine, Col: curCol}
		case "+^":
			l.advance(); l.advance()
			return Token{Type: TokBitXor, Literal: "+^", Line: curLine, Col: curCol}
		case "+<":
			l.advance(); l.advance()
			return Token{Type: TokBitShiftL, Literal: "+<", Line: curLine, Col: curCol}
		case "+>":
			l.advance(); l.advance()
			return Token{Type: TokBitShiftR, Literal: "+>", Line: curLine, Col: curCol}
		case "%%":
			l.advance(); l.advance()
			return Token{Type: TokDivisible, Literal: "%%", Line: curLine, Col: curCol}
		case "..":
			l.advance(); l.advance()
			return Token{Type: TokDotDot, Literal: "..", Line: curLine, Col: curCol}
		case "||":
			l.advance(); l.advance()
			return Token{Type: TokOr, Literal: "||", Line: curLine, Col: curCol}
		}
	}

	// Single-character tokens
	l.advance()
	switch ch {
	case '+':
		return Token{Type: TokPlus, Literal: "+", Line: curLine, Col: curCol}
	case '-':
		return Token{Type: TokMinus, Literal: "-", Line: curLine, Col: curCol}
	case '*', '×':
		return Token{Type: TokStar, Literal: "*", Line: curLine, Col: curCol}
	case '/', '÷':
		return Token{Type: TokSlash, Literal: "/", Line: curLine, Col: curCol}
	case '≤':
		return Token{Type: TokLessEq, Literal: "<=", Line: curLine, Col: curCol}
	case '≥':
		return Token{Type: TokGreaterEq, Literal: ">=", Line: curLine, Col: curCol}
	case '≠':
		return Token{Type: TokNotEqual, Literal: "!=", Line: curLine, Col: curCol}
	case '∈':
		return Token{Type: TokElem, Literal: "∈", Line: curLine, Col: curCol}
	case '∉':
		return Token{Type: TokNotElem, Literal: "∉", Line: curLine, Col: curCol}
	case '∩':
		return Token{Type: TokIntersect, Literal: "∩", Line: curLine, Col: curCol}
	case '∪':
		return Token{Type: TokUnionOp, Literal: "∪", Line: curLine, Col: curCol}
	case '%':
		return Token{Type: TokPercent, Literal: "%", Line: curLine, Col: curCol}
	case '=':
		return Token{Type: TokAssign, Literal: "=", Line: curLine, Col: curCol}
	case '<':
		return Token{Type: TokAngleL, Literal: "<", Line: curLine, Col: curCol}
	case '>':
		return Token{Type: TokAngleR, Literal: ">", Line: curLine, Col: curCol}
	case '!':
		return Token{Type: TokNot, Literal: "!", Line: curLine, Col: curCol}
	case '~':
		return Token{Type: TokConcat, Literal: "~", Line: curLine, Col: curCol}
	case ';':
		return Token{Type: TokSemicolon, Literal: ";", Line: curLine, Col: curCol}
	case ':':
		return Token{Type: TokColon, Literal: ":", Line: curLine, Col: curCol}
	case ',':

		return Token{Type: TokComma, Literal: ",", Line: curLine, Col: curCol}
	case '.':
		return Token{Type: TokDot, Literal: ".", Line: curLine, Col: curCol}
	case '(':
		return Token{Type: TokLParen, Literal: "(", Line: curLine, Col: curCol}
	case ')':
		return Token{Type: TokRParen, Literal: ")", Line: curLine, Col: curCol}
	case '{':
		return Token{Type: TokLBrace, Literal: "{", Line: curLine, Col: curCol}
	case '}':
		return Token{Type: TokRBrace, Literal: "}", Line: curLine, Col: curCol}
	case '[':
		return Token{Type: TokLBracket, Literal: "[", Line: curLine, Col: curCol}
	case ']':
		return Token{Type: TokRBracket, Literal: "]", Line: curLine, Col: curCol}
	case '&':
		return Token{Type: TokSubRef, Literal: "&", Line: curLine, Col: curCol}
	case '?':
		return Token{Type: TokQuestion, Literal: "?", Line: curLine, Col: curCol}

	default:
		return Token{Type: TokError, Literal: fmt.Sprintf("unexpected character %c", ch), Line: curLine, Col: curCol}
	}
}

func (l *Lexer) advance() {
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.advance()
		} else if ch == '#' {
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.advance()
			}
		} else {
			break
		}
	}
}

func (l *Lexer) scanVariable() Token {
	curLine := l.line
	curCol := l.col
	sigil := l.src[l.pos]
	l.advance()

	var sb strings.Builder
	sb.WriteRune(sigil)

	if l.pos < len(l.src) && (l.src[l.pos] == '*' || l.src[l.pos] == '!' || l.src[l.pos] == '?') {
		sb.WriteRune(l.src[l.pos])
		l.advance()
	}

	for l.pos < len(l.src) && (unicode.IsLetter(l.src[l.pos]) || unicode.IsDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		sb.WriteRune(l.src[l.pos])
		l.advance()
	}

	literal := sb.String()
	switch sigil {
	case '$':
		return Token{Type: TokScalar, Literal: literal, Line: curLine, Col: curCol}
	case '@':
		return Token{Type: TokArray, Literal: literal, Line: curLine, Col: curCol}
	case '%':
		return Token{Type: TokHash, Literal: literal, Line: curLine, Col: curCol}
	default:
		return Token{Type: TokScalar, Literal: literal, Line: curLine, Col: curCol}
	}
}

func (l *Lexer) scanNumber() Token {
	curLine := l.line
	curCol := l.col
	var sb strings.Builder
	isFloat := false

	if l.src[l.pos] == '0' && l.pos+1 < len(l.src) {
		next := l.src[l.pos+1]
		if next == 'x' || next == 'X' {
			sb.WriteString("0x")
			l.advance()
			l.advance()
			for l.pos < len(l.src) && ((l.src[l.pos] >= '0' && l.src[l.pos] <= '9') || (l.src[l.pos] >= 'a' && l.src[l.pos] <= 'f') || (l.src[l.pos] >= 'A' && l.src[l.pos] <= 'F') || l.src[l.pos] == '_') {
				if l.src[l.pos] != '_' {
					sb.WriteRune(l.src[l.pos])
				}
				l.advance()
			}
			return Token{Type: TokInt, Literal: sb.String(), Line: curLine, Col: curCol}
		}
		if next == 'b' || next == 'B' {
			sb.WriteString("0b")
			l.advance()
			l.advance()
			for l.pos < len(l.src) && (l.src[l.pos] == '0' || l.src[l.pos] == '1' || l.src[l.pos] == '_') {
				if l.src[l.pos] != '_' {
					sb.WriteRune(l.src[l.pos])
				}
				l.advance()
			}
			return Token{Type: TokInt, Literal: sb.String(), Line: curLine, Col: curCol}
		}
	}

	for l.pos < len(l.src) && (unicode.IsDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		if l.src[l.pos] != '_' {
			sb.WriteRune(l.src[l.pos])
		}
		l.advance()
	}

	if l.pos+1 < len(l.src) && l.src[l.pos] == '.' && unicode.IsDigit(l.src[l.pos+1]) {
		isFloat = true
		sb.WriteRune('.')
		l.advance()
		for l.pos < len(l.src) && (unicode.IsDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
			if l.src[l.pos] != '_' {
				sb.WriteRune(l.src[l.pos])
			}
			l.advance()
		}
	}

	lit := sb.String()
	if isFloat {
		return Token{Type: TokFloat, Literal: lit, Line: curLine, Col: curCol}
	}
	return Token{Type: TokInt, Literal: lit, Line: curLine, Col: curCol}
}

func (l *Lexer) scanString(quote rune) Token {
	curLine := l.line
	curCol := l.col
	l.advance() // skip opening quote

	var sb strings.Builder
	hasInterp := false
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == quote {
			l.advance() // skip closing quote
			break
		}
		if ch == '\\' && l.pos+1 < len(l.src) {
			l.advance()
			next := l.src[l.pos]
			switch next {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '\\':
				sb.WriteRune('\\')
			case quote:
				sb.WriteRune(quote)
			default:
				sb.WriteRune(next)
			}
			l.advance()
			continue
		}
		// Detect interpolation markers in double-quoted strings
		if quote == '"' && (ch == '$' || ch == '{') {
			hasInterp = true
		}
		sb.WriteRune(ch)
		l.advance()
	}

	tokType := TokString
	if quote == '"' && hasInterp {
		tokType = TokInterpString
	}
	return Token{Type: tokType, Literal: sb.String(), Line: curLine, Col: curCol}
}


func (l *Lexer) scanIdentOrKeyword() Token {
	curLine := l.line
	curCol := l.col
	var sb strings.Builder

	for l.pos < len(l.src) && (unicode.IsLetter(l.src[l.pos]) || unicode.IsDigit(l.src[l.pos]) || l.src[l.pos] == '_' || l.src[l.pos] == ':') {
		sb.WriteRune(l.src[l.pos])
		l.advance()
	}

	lit := sb.String()
	// Check for operator syntax: infix:<+>, prefix:<->, postfix:<!>, postcircumfix:<[ ]>, circumfix:<[ ]>
	if (strings.HasPrefix(lit, "infix:") || strings.HasPrefix(lit, "prefix:") || strings.HasPrefix(lit, "postfix:") || strings.HasPrefix(lit, "postcircumfix:") || strings.HasPrefix(lit, "circumfix:") || lit == "infix" || lit == "prefix" || lit == "postfix" || lit == "postcircumfix") && l.pos < len(l.src) && (l.src[l.pos] == '<' || (l.pos+1 < len(l.src) && l.src[l.pos] == ':' && l.src[l.pos+1] == '<')) {
		if l.src[l.pos] == ':' {
			sb.WriteRune(':')
			l.advance()
		}
		if l.pos < len(l.src) && l.src[l.pos] == '<' {
			sb.WriteRune('<')
			l.advance()
			for l.pos < len(l.src) && l.src[l.pos] != '>' {
				sb.WriteRune(l.src[l.pos])
				l.advance()
			}
			if l.pos < len(l.src) && l.src[l.pos] == '>' {
				sb.WriteRune('>')
				l.advance()
			}
		}
		return Token{Type: TokIdent, Literal: sb.String(), Line: curLine, Col: curCol}
	}

	switch lit {

	case "my":
		return Token{Type: TokMy, Literal: lit, Line: curLine, Col: curCol}
	case "our":
		return Token{Type: TokOur, Literal: lit, Line: curLine, Col: curCol}
	case "sub":
		return Token{Type: TokSub, Literal: lit, Line: curLine, Col: curCol}
	case "multi":
		return Token{Type: TokMulti, Literal: lit, Line: curLine, Col: curCol}
	case "return":
		return Token{Type: TokReturn, Literal: lit, Line: curLine, Col: curCol}
	case "if":
		return Token{Type: TokIf, Literal: lit, Line: curLine, Col: curCol}
	case "elsif":
		return Token{Type: TokElsif, Literal: lit, Line: curLine, Col: curCol}
	case "else":
		return Token{Type: TokElse, Literal: lit, Line: curLine, Col: curCol}
	case "unless":
		return Token{Type: TokUnless, Literal: lit, Line: curLine, Col: curCol}
	case "while":
		return Token{Type: TokWhile, Literal: lit, Line: curLine, Col: curCol}
	case "until":
		return Token{Type: TokUntil, Literal: lit, Line: curLine, Col: curCol}
	case "for":
		return Token{Type: TokFor, Literal: lit, Line: curLine, Col: curCol}
	case "loop":
		return Token{Type: TokLoop, Literal: lit, Line: curLine, Col: curCol}
	case "use":
		return Token{Type: TokUse, Literal: lit, Line: curLine, Col: curCol}
	case "eq":
		return Token{Type: TokEqStr, Literal: lit, Line: curLine, Col: curCol}
	case "ne":
		return Token{Type: TokNeStr, Literal: lit, Line: curLine, Col: curCol}
	case "lt":
		return Token{Type: TokLtStr, Literal: lit, Line: curLine, Col: curCol}
	case "gt":
		return Token{Type: TokGtStr, Literal: lit, Line: curLine, Col: curCol}
	case "and":
		return Token{Type: TokAnd, Literal: lit, Line: curLine, Col: curCol}
	case "or":
		return Token{Type: TokOr, Literal: lit, Line: curLine, Col: curCol}
	case "not":
		return Token{Type: TokNot, Literal: lit, Line: curLine, Col: curCol}
	case "x":
		return Token{Type: TokRepeat, Literal: lit, Line: curLine, Col: curCol}
	case "xx":
		return Token{Type: TokListRepeat, Literal: lit, Line: curLine, Col: curCol}
	case "div":
		return Token{Type: TokDiv, Literal: lit, Line: curLine, Col: curCol}
	case "mod":
		return Token{Type: TokMod, Literal: lit, Line: curLine, Col: curCol}
	case "min":
		return Token{Type: TokMin, Literal: lit, Line: curLine, Col: curCol}
	case "max":
		return Token{Type: TokMax, Literal: lit, Line: curLine, Col: curCol}
	case "given":
		return Token{Type: TokGiven, Literal: lit, Line: curLine, Col: curCol}
	case "when":
		return Token{Type: TokWhen, Literal: lit, Line: curLine, Col: curCol}
	case "default":
		return Token{Type: TokDefault, Literal: lit, Line: curLine, Col: curCol}
	case "enum":
		return Token{Type: TokEnum, Literal: lit, Line: curLine, Col: curCol}
	case "last":
		return Token{Type: TokLast, Literal: lit, Line: curLine, Col: curCol}
	case "subset":
		return Token{Type: TokSubset, Literal: lit, Line: curLine, Col: curCol}
	case "returns":
		return Token{Type: TokReturns, Literal: lit, Line: curLine, Col: curCol}
	case "struct":
		return Token{Type: TokStruct, Literal: lit, Line: curLine, Col: curCol}
	case "union":
		return Token{Type: TokUnion, Literal: lit, Line: curLine, Col: curCol}
	case "where":
		return Token{Type: TokWhere, Literal: lit, Line: curLine, Col: curCol}
	case "assert":
		return Token{Type: TokAssert, Literal: lit, Line: curLine, Col: curCol}
	case "before":
		return Token{Type: TokBefore, Literal: lit, Line: curLine, Col: curCol}
	case "after":
		return Token{Type: TokAfter, Literal: lit, Line: curLine, Col: curCol}
	case "around":
		return Token{Type: TokAround, Literal: lit, Line: curLine, Col: curCol}

	default:
		return Token{Type: TokIdent, Literal: lit, Line: curLine, Col: curCol}
	}
}
