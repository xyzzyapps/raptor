package raptor

import (
	"fmt"
	"strconv"
	"strings"
)


// Parser parses tokens into an AST for Raku5.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a new Parser instance.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

// Parse parses the token stream into a Program AST.
func (p *Parser) Parse() (*Program, error) {
	var stmts []Stmt
	for !p.isAtEnd() {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	return &Program{Stmts: stmts}, nil
}

// ParseProgram tokenizes and parses a source code string into a Program AST.
func ParseProgram(source string) (*Program, error) {
	lexer := NewLexer(source)
	var tokens []Token
	for {
		tok := lexer.NextToken()
		if tok.Type == TokError {
			return nil, fmt.Errorf("lex error at line %d, col %d: %s", tok.Line, tok.Col, tok.Literal)
		}
		tokens = append(tokens, tok)
		if tok.Type == TokEOF {
			break
		}
	}
	parser := NewParser(tokens)
	return parser.Parse()
}


func (p *Parser) parseStatement() (Stmt, error) {
	tok := p.peek()

	switch tok.Type {
	case TokSemicolon:
		p.advance()
		return nil, nil

	case TokMy, TokOur:
		return p.parseVarDecl()

	case TokMulti:
		p.advance()
		if p.peek().Type == TokSub {
			return p.parseSubDecl(true)
		}
		return nil, fmt.Errorf("line %d: expected 'sub' after 'multi', got %s", p.peek().Line, p.peek().Literal)

	case TokSub:
		return p.parseSubDecl(false)

	case TokReturn:
		p.advance()
		if p.peek().Type == TokSemicolon {
			p.advance()
			return &ReturnStmt{Value: nil}, nil
		}
		val, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		p.match(TokSemicolon)
		return &ReturnStmt{Value: val}, nil

	case TokIf:
		return p.parseIf()

	case TokUnless:
		return p.parseUnless()

	case TokWhile:
		return p.parseWhile(false)

	case TokUntil:
		return p.parseWhile(true)

	case TokFor:
		return p.parseFor()

	case TokLoop:
		return p.parseLoop()

	case TokUse:
		return p.parseUse()

	case TokGiven:
		return p.parseGiven()

	case TokEnum:
		return p.parseEnum()

	case TokSubset:
		return p.parseSubsetDecl()

	case TokStruct, TokUnion:
		return p.parseStructDecl()

	case TokAssert:
		return p.parseAssert()

	case TokBefore, TokAfter, TokAround:
		return p.parseAdviceHook()

	case TokLBrace:
		return p.parseBlock()

	default:
		if p.peek().Type == TokIdent && p.peek().Literal == "take" {
			p.advance()
			val, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			p.match(TokSemicolon)
			return &TakeStmt{Value: val}, nil
		}

		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		p.match(TokSemicolon)
		return &ExprStmt{Expr: expr}, nil
	}
}


func (p *Parser) parseVarDecl() (Stmt, error) {
	scopeTok := p.advance()
	scope := scopeTok.Literal

	var typeName string
	if p.isTypeToken(p.peek().Type) || (p.peek().Type == TokIdent && (p.peekNext().Type == TokScalar || p.peekNext().Type == TokArray || p.peekNext().Type == TokHash)) {
		typeTok := p.advance()
		typeName = typeTok.Literal
	}

	nameTok := p.peek()
	if nameTok.Type != TokScalar && nameTok.Type != TokArray && nameTok.Type != TokHash {
		return nil, fmt.Errorf("line %d: expected variable name after %s, got %s", nameTok.Line, scope, nameTok.Literal)
	}
	p.advance()

	var whereExpr Expr
	if p.peek().Type == TokWhere {
		p.advance()
		if p.peek().Type == TokLBrace {
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			whereExpr = &ClosureExpr{Body: body}
		} else {
			we, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			whereExpr = we
		}
	}

	var init Expr
	if p.peek().Type == TokAssign {
		p.advance()
		val, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		init = val
	}

	p.match(TokSemicolon)
	return &VarDeclStmt{
		Scope: scope,
		Type:  typeName,
		Name:  nameTok.Literal,
		Where: whereExpr,
		Value: init,
	}, nil
}

func (p *Parser) parseSubDecl(isMulti bool) (Stmt, error) {
	p.consume(TokSub)
	nameTok := p.peek()
	name := ""
	if nameTok.Type == TokIdent {
		name = nameTok.Literal
		p.advance()
	}

	var params []Param
	if p.peek().Type == TokLParen {
		p.advance()
		for p.peek().Type != TokRParen && !p.isAtEnd() {
			// Array destructuring parameter: [$h, *@t]
			if p.peek().Type == TokLBracket {
				p.advance()
				var dParams []Param
				for p.peek().Type != TokRBracket && !p.isAtEnd() {
					isSlurpy := false
					if p.peek().Type == TokStar || p.peek().Literal == "*" {
						p.advance()
						isSlurpy = true
					}
					dpTok := p.advance()
					dParams = append(dParams, Param{
						Name:     dpTok.Literal,
						IsSlurpy: isSlurpy,
					})
					if p.peek().Type == TokComma {
						p.advance()
					}
				}
				p.consume(TokRBracket)
				params = append(params, Param{
					Name:        fmt.Sprintf("$_destruct_arr_%d", len(params)),
					Type:        "Array",
					DestructArr: dParams,
				})
				if p.peek().Type == TokComma {
					p.advance()
				}
				continue
			}

			// Hash destructuring parameter: :{:$name, :$age}
			if p.peek().Type == TokLBrace || (p.peek().Type == TokColon && p.peekNext().Type == TokLBrace) {
				if p.peek().Type == TokColon {
					p.advance()
				}
				p.advance()
				var dParams []Param
				for p.peek().Type != TokRBrace && !p.isAtEnd() {
					if p.peek().Type == TokColon {
						p.advance()
					}
					dpTok := p.advance()
					dParams = append(dParams, Param{
						Name: dpTok.Literal,
					})
					if p.peek().Type == TokComma {
						p.advance()
					}
				}
				p.consume(TokRBrace)
				params = append(params, Param{
					Name:         fmt.Sprintf("$_destruct_hash_%d", len(params)),
					Type:         "Hash",
					DestructHash: dParams,
				})
				if p.peek().Type == TokComma {
					p.advance()
				}
				continue
			}

			var paramType string
			if p.isTypeToken(p.peek().Type) || (p.peek().Type == TokIdent && (p.peekNext().Type == TokScalar || p.peekNext().Type == TokArray || p.peekNext().Type == TokHash || p.peekNext().Type == TokSubRef)) {
				paramType = p.advance().Literal
			}
			paramTok := p.peek()
			if paramTok.Type == TokScalar || paramTok.Type == TokArray || paramTok.Type == TokHash || paramTok.Type == TokSubRef {
				pName := paramTok.Literal
				p.advance()

				var paramWhere Expr
				if p.peek().Type == TokWhere {
					p.advance()
					if p.peek().Type == TokLBrace {
						body, err := p.parseBlock()
						if err != nil {
							return nil, err
						}
						paramWhere = &ClosureExpr{Body: body}
					} else {
						we, err := p.parseExpression(0)
						if err != nil {
							return nil, err
						}
						paramWhere = we
					}
				}

				params = append(params, Param{
					Name:  pName,
					Type:  paramType,
					Where: paramWhere,
				})
			} else {
				return nil, fmt.Errorf("line %d: expected parameter name, got %s", paramTok.Line, paramTok.Literal)
			}
			if p.peek().Type == TokComma {
				p.advance()
			}
		}
		p.consume(TokRParen)
	}


	// Check for returns <Type> or --> <Type>
	var retType string
	if p.peek().Type == TokReturns {
		p.advance()
		retType = p.advance().Literal
	} else if p.peek().Type == TokArrow {
		p.advance()
		retType = p.advance().Literal
	}

	// Check for traits: is native('lib.dll') is symbol('sym')
	var nativeLib string
	var nativeSym string
	for p.peek().Type == TokIdent && p.peek().Literal == "is" {
		p.advance() // skip 'is'
		traitNameTok := p.advance()
		traitName := traitNameTok.Literal
		if traitName == "native" {
			if p.peek().Type == TokLParen {
				p.advance()
				libTok := p.advance()
				nativeLib = libTok.Literal
				p.match(TokRParen)
			} else {
				nativeLib = "native"
			}
		} else if traitName == "symbol" {
			if p.peek().Type == TokLParen {
				p.advance()
				symTok := p.advance()
				nativeSym = symTok.Literal
				p.match(TokRParen)
			}
		} else if traitName == "repr" {
			if p.peek().Type == TokLParen {
				p.advance()
				p.advance()
				p.match(TokRParen)
			}
		}
	}

	if nativeLib != "" {
		if p.peek().Type == TokLBrace {
			p.advance()
			for p.peek().Type != TokRBrace && !p.isAtEnd() {
				p.advance()
			}
			p.consume(TokRBrace)
		} else {
			p.match(TokSemicolon)
		}
		if nativeSym == "" {
			nativeSym = name
		}
		return &NativeSubDeclStmt{
			Name:       name,
			Params:     params,
			ReturnType: retType,
			Library:    nativeLib,
			Symbol:     nativeSym,
		}, nil
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &SubDeclStmt{
		IsMulti: isMulti,
		Name:    name,
		Params:  params,
		Body:    body,
	}, nil
}


func (p *Parser) parseIf() (Stmt, error) {
	p.consume(TokIf)
	cond, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	thenBlock, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var elsifConds []Expr
	var elsifThen []*BlockStmt

	for p.peek().Type == TokElsif {
		p.advance()
		eCond, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		eBlock, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		elsifConds = append(elsifConds, eCond)
		elsifThen = append(elsifThen, eBlock)
	}

	var elseBlock *BlockStmt
	if p.peek().Type == TokElse {
		p.advance()
		b, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		elseBlock = b
	}

	return &IfStmt{
		Condition:  cond,
		ThenBranch: thenBlock,
		ElsifConds: elsifConds,
		ElsifThen:  elsifThen,
		ElseBranch: elseBlock,
	}, nil
}

func (p *Parser) parseUnless() (Stmt, error) {
	p.consume(TokUnless)
	cond, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &UnlessStmt{Condition: cond, Body: body}, nil
}

func (p *Parser) parseWhile(isUntil bool) (Stmt, error) {
	p.advance() // consume while or until
	cond, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &WhileStmt{IsUntil: isUntil, Condition: cond, Body: body}, nil
}

func (p *Parser) parseFor() (Stmt, error) {
	p.consume(TokFor)
	iterable, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}

	var varName string
	if p.peek().Type == TokArrow {
		p.advance()
		vTok := p.peek()
		if vTok.Type == TokScalar || vTok.Type == TokArray || vTok.Type == TokHash {
			varName = vTok.Literal
			p.advance()
		}
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ForStmt{Iterable: iterable, VarName: varName, Body: body}, nil
}

func (p *Parser) parseLoop() (Stmt, error) {
	p.consume(TokLoop)
	p.consume(TokLParen)

	var initExpr Expr
	if p.peek().Type == TokMy {
		p.advance()
		vTok := p.peek()
		p.advance()
		p.consume(TokAssign)
		val, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		initExpr = &AssignStmt{Target: &VarExpr{Name: vTok.Literal}, Op: "=", Value: val}
	} else if p.peek().Type != TokSemicolon {
		e, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		initExpr = e
	}
	p.consume(TokSemicolon)

	var condExpr Expr
	if p.peek().Type != TokSemicolon {
		e, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		condExpr = e
	}
	p.consume(TokSemicolon)

	var stepExpr Expr
	if p.peek().Type != TokRParen {
		e, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		stepExpr = e
	}
	p.consume(TokRParen)

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &LoopStmt{Init: initExpr, Cond: condExpr, Step: stepExpr, Body: body}, nil
}

func (p *Parser) parseUse() (Stmt, error) {
	p.consume(TokUse)
	modTok := p.peek()
	p.advance()
	modName := modTok.Literal

	fromName := ""
	if p.peek().Type == TokIdent && p.peek().Literal == ":from" {
		p.advance()
		if p.peek().Type == TokAngleL {
			p.advance()
			fromName = p.peek().Literal
			p.advance()
			p.match(TokAngleR)
		}
	}

	p.match(TokSemicolon)
	return &UseStmt{Module: modName, From: fromName}, nil
}

func (p *Parser) parseBlock() (*BlockStmt, error) {
	if err := p.consume(TokLBrace); err != nil {
		return nil, err
	}
	var stmts []Stmt
	for p.peek().Type != TokRBrace && !p.isAtEnd() {
		s, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if s != nil {
			stmts = append(stmts, s)
		}
	}
	if err := p.consume(TokRBrace); err != nil {
		return nil, err
	}
	return &BlockStmt{Stmts: stmts}, nil
}

func (p *Parser) parseExpression(minPrec int) (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()

		// Assignment operators: =, +=, -=, ~=, //=
		if tok.Type == TokAssign || tok.Type == TokAddAssign || tok.Type == TokSubAssign || tok.Type == TokConcatAssign || tok.Type == TokDefinedOrAssign {
			if minPrec > 1 {
				break
			}
			p.advance()
			op := tok.Literal
			val, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			left = &AssignStmt{Target: left, Op: op, Value: val}
			continue
		}

		// Raku Ternary: $cond ?? $then !! $else
		if tok.Type == TokQuestionQuestion {
			if minPrec > 2 {
				break
			}
			p.advance()
			thenExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			if err := p.consume(TokExclamationExclamation); err != nil {
				return nil, err
			}
			elseExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			left = &TernaryExpr{Cond: left, Then: thenExpr, Else: elseExpr}
			continue
		}

		// Perl5 / C Ternary: $cond ? $then : $else
		if tok.Type == TokQuestion {
			if minPrec > 2 {
				break
			}
			p.advance()
			thenExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			if err := p.consume(TokColon); err != nil {
				return nil, err
			}
			elseExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			left = &TernaryExpr{Cond: left, Then: thenExpr, Else: elseExpr}
			continue
		}

		// Array index access
		if tok.Type == TokLBracket {
			p.advance()
			idxExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			p.consume(TokRBracket)
			left = &IndexExpr{Array: left, Index: idxExpr}
			continue
		}

		// Function call
		if tok.Type == TokLParen {
			p.advance()
			var args []Expr
			for p.peek().Type != TokRParen && !p.isAtEnd() {
				arg, err := p.parseExpression(0)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				if p.peek().Type == TokComma {
					p.advance()
				}
			}
			p.consume(TokRParen)
			left = &CallExpr{Callee: left, Args: args}
			continue
		}

		// Dot method call / UFCS: $obj.method(args...) or $obj.method
		if tok.Type == TokDot {
			p.advance()
			methTok := p.advance()
			methodName := methTok.Literal


			var args []Expr
			if p.peek().Type == TokLParen {
				p.advance()
				for p.peek().Type != TokRParen && !p.isAtEnd() {
					arg, err := p.parseExpression(0)
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.peek().Type == TokComma {
						p.advance()
					}
				}
				p.consume(TokRParen)
			}
			left = &MethodCallExpr{Target: left, Method: methodName, Args: args}
			continue
		}

		// Hash access %h<key>, $obj<key>, $arr[0]<key>, $h<k1><k2>
		if tok.Type == TokAngleL {
			if p.pos+2 < len(p.tokens) && p.tokens[p.pos+2].Type == TokAngleR {
				p.advance()
				keyTok := p.advance()
				p.consume(TokAngleR)
				left = &HashAccessExpr{Hash: left, Key: &LiteralExpr{Type: TokString, Value: keyTok.Literal}}
				continue
			}
		}

		// Hash access %h{"key"} or %h{$k} or $h[0]{"key"}
		if tok.Type == TokLBrace {
			isHashTarget := false
			switch v := left.(type) {
			case *VarExpr:
				if strings.HasPrefix(v.Name, "%") {
					isHashTarget = true
				}
			case *IndexExpr, *HashAccessExpr:
				isHashTarget = true
			}

			if isHashTarget {
				p.advance()
				keyExpr, err := p.parseExpression(0)
				if err != nil {
					return nil, err
				}
				p.consume(TokRBrace)
				left = &HashAccessExpr{Hash: left, Key: keyExpr}
				continue
			}
		}


		// Smart match operator ~~
		if tok.Type == TokSmartMatch {
			p.advance()
			right, err := p.parseExpression(1)
			if err != nil {
				return nil, err
			}
			left = &SmartMatchExpr{Left: left, Right: right}
			continue
		}

		prec := getPrecedence(tok)
		if prec <= minPrec {
			break
		}

		p.advance()
		op := tok.Literal
		right, err := p.parseExpression(prec)
		if err != nil {
			return nil, err
		}

		// Chained comparisons: 1 < $x < 10 -> ChainedCompExpr
		if isComparisonOp(tok.Type) {
			if chain, ok := left.(*ChainedCompExpr); ok {
				chain.Ops = append(chain.Ops, op)
				chain.Exprs = append(chain.Exprs, right)
				continue
			}
			if bin, ok := left.(*BinaryExpr); ok && isComparisonOpStr(bin.Op) {
				left = &ChainedCompExpr{
					Exprs: []Expr{bin.Left, bin.Right, right},
					Ops:   []string{bin.Op, op},
				}
				continue
			}
		}

		left = &BinaryExpr{Left: left, Op: op, Right: right}
	}

	return left, nil
}


func (p *Parser) parsePrimary() (Expr, error) {
	tok := p.peek()

	switch tok.Type {
	case TokMinus:
		p.advance()
		if p.peek().Type == TokInt {
			intTok := p.advance()
			val, _ := strconv.ParseInt(intTok.Literal, 0, 64)
			return &LiteralExpr{Type: TokInt, Value: -val}, nil
		}
		if p.peek().Type == TokFloat {
			floatTok := p.advance()
			val, _ := strconv.ParseFloat(floatTok.Literal, 64)
			return &LiteralExpr{Type: TokFloat, Value: -val}, nil
		}
		op := tok.Literal
		right, err := p.parseExpression(8)
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: op, Right: right}, nil

	case TokPlus, TokNot, TokFileTest:
		p.advance()
		op := tok.Literal
		right, err := p.parseExpression(8)
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: op, Right: right}, nil


	case TokInt:
		p.advance()
		val, _ := strconv.ParseInt(tok.Literal, 0, 64)
		return &LiteralExpr{Type: TokInt, Value: val}, nil


	case TokFloat:
		p.advance()
		val, _ := strconv.ParseFloat(tok.Literal, 64)
		return &LiteralExpr{Type: TokFloat, Value: val}, nil

	case TokSlash:
		p.advance() // consume opening '/'
		var pattern strings.Builder
		for !p.isAtEnd() && p.peek().Type != TokSlash {
			pattern.WriteString(p.advance().Literal)
		}
		p.consume(TokSlash) // consume closing '/'
		return &LiteralExpr{Type: TokString, Value: pattern.String()}, nil

	case TokString:
		p.advance()
		return &LiteralExpr{Type: TokString, Value: tok.Literal}, nil

	case TokInterpString:
		p.advance()
		return p.parseInterpString(tok.Literal)

	case TokScalar, TokArray, TokHash:
		p.advance()
		return &VarExpr{Name: tok.Literal}, nil

	case TokSubRef:
		p.advance()
		idTok := p.peek()
		if idTok.Type == TokIdent {
			p.advance()
			return &VarExpr{Name: "&" + idTok.Literal}, nil
		}
		return &VarExpr{Name: "&"}, nil

	case TokIdent:
		p.advance()
		name := tok.Literal
		if name == "start" && p.peek().Type == TokLBrace {
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			closure := &ClosureExpr{Body: body}
			return &CallExpr{Callee: &VarExpr{Name: "start"}, Args: []Expr{closure}}, nil
		}
		if name == "gather" && p.peek().Type == TokLBrace {
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			return &GatherExpr{Body: body}, nil
		}

		// Function call without parens in statement or argument context (e.g. say 1, 2)
		if p.peek().Type != TokDot && p.peek().Type != TokAssign && p.peek().Type != TokLParen && p.peek().Type != TokLBracket && p.peek().Type != TokAngleL && p.peek().Type != TokSemicolon && p.peek().Type != TokRParen && p.peek().Type != TokRBrace && p.peek().Type != TokRBracket && p.peek().Type != TokComma && !p.isBinaryOp(p.peek()) {



			var args []Expr
			for p.peek().Type != TokSemicolon && p.peek().Type != TokRParen && p.peek().Type != TokRBrace && !p.isAtEnd() {
				arg, err := p.parseExpression(2)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				if p.peek().Type == TokComma {
					p.advance()
				} else {
					break
				}
			}
			return &CallExpr{Callee: &VarExpr{Name: name}, Args: args}, nil
		}
		return &VarExpr{Name: name}, nil

	case TokColon:
		p.advance()
		nameTok := p.advance()
		keyName := nameTok.Literal
		for p.peek().Type == TokMinus && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TokIdent {
			p.advance() // consume '-'
			nextIdent := p.advance()
			keyName += "-" + nextIdent.Literal
		}
		return &LiteralExpr{Type: TokString, Value: keyName}, nil

	case TokLParen:

		p.advance()
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		p.consume(TokRParen)
		return expr, nil

	case TokLBracket:
		p.advance()
		var elems []Expr
		for p.peek().Type != TokRBracket && !p.isAtEnd() {
			elem, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			elems = append(elems, elem)
			if p.peek().Type == TokComma {
				p.advance()
			}
		}
		p.consume(TokRBracket)
		return &ArrayLiteralExpr{Elements: elems}, nil

	case TokLBrace:
		p.advance()
		var pairs [][2]Expr
		for p.peek().Type != TokRBrace && !p.isAtEnd() {
			kExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			if p.peek().Type == TokFatArrow || p.peek().Type == TokComma {
				p.advance()
			}
			vExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, [2]Expr{kExpr, vExpr})
			if p.peek().Type == TokComma {
				p.advance()
			}
		}
		p.consume(TokRBrace)
		return &HashLiteralExpr{Pairs: pairs}, nil

	case TokSub:
		p.advance()
		var params []Param
		if p.peek().Type == TokLParen {
			p.advance()
			for p.peek().Type != TokRParen && !p.isAtEnd() {
				var pType string
				if p.isTypeToken(p.peek().Type) {
					pType = p.advance().Literal
				}
				paramTok := p.peek()
				if paramTok.Type == TokScalar || paramTok.Type == TokArray || paramTok.Type == TokHash {
					params = append(params, Param{Name: paramTok.Literal, Type: pType})
					p.advance()
				}
				if p.peek().Type == TokComma {
					p.advance()
				}
			}
			p.consume(TokRParen)
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &ClosureExpr{Params: params, Body: body}, nil

	case TokArrow:
		p.advance()
		var params []Param
		for p.isTypeToken(p.peek().Type) || p.peek().Type == TokScalar || p.peek().Type == TokArray || p.peek().Type == TokHash {
			var pType string
			if p.isTypeToken(p.peek().Type) {
				pType = p.advance().Literal
			}
			pName := p.advance().Literal
			params = append(params, Param{Name: pName, Type: pType})
			if p.peek().Type == TokComma {
				p.advance()
			}
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &ClosureExpr{Params: params, Body: body}, nil

	default:
		return nil, fmt.Errorf("line %d: unexpected expression token %v (%q)", tok.Line, tok.Type, tok.Literal)
	}

}

// parseGiven parses: given $expr { when $match { } ... default { } }
func (p *Parser) parseGiven() (Stmt, error) {
	p.advance() // skip 'given'
	topic, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	p.consume(TokLBrace)

	var whens []WhenClause
	var defaultBlock *BlockStmt

	for p.peek().Type != TokRBrace && !p.isAtEnd() {
		if p.peek().Type == TokWhen {
			p.advance()
			matchExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			whens = append(whens, WhenClause{Match: matchExpr, Body: body})
		} else if p.peek().Type == TokDefault {
			p.advance()
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			defaultBlock = body
		} else {
			break
		}
	}
	p.consume(TokRBrace)
	return &GivenStmt{Topic: topic, Whens: whens, Default: defaultBlock}, nil
}

// parseEnum parses: enum Color <Red Green Blue>; or enum Dir (N => 0, S => 1);
func (p *Parser) parseEnum() (Stmt, error) {
	p.advance() // skip 'enum'
	nameTok := p.advance()
	enumName := nameTok.Literal

	var values []EnumValue

	if p.peek().Type == TokAngleL {
		// enum Color <Red Green Blue>
		p.advance()
		idx := int64(0)
		for p.peek().Type != TokAngleR && !p.isAtEnd() {
			valTok := p.advance()
			values = append(values, EnumValue{Name: valTok.Literal, Index: idx})
			idx++
		}
		p.match(TokAngleR)
	} else if p.peek().Type == TokLParen {
		// enum Dir (N => 0, S => 1)
		p.advance()
		for p.peek().Type != TokRParen && !p.isAtEnd() {
			valTok := p.advance()
			var idx int64
			if p.peek().Type == TokFatArrow {
				p.advance()
				idxTok := p.advance()
				idx, _ = strconv.ParseInt(idxTok.Literal, 10, 64)
			}
			values = append(values, EnumValue{Name: valTok.Literal, Index: idx})
			p.match(TokComma)
		}
		p.consume(TokRParen)
	}

	p.match(TokSemicolon)
	return &EnumDeclStmt{Name: enumName, Values: values}, nil
}

// parseInterpString parses interpolation markers ($var, {expr}) in double-quoted strings.
func (p *Parser) parseInterpString(raw string) (Expr, error) {
	runes := []rune(raw)
	var parts []Expr
	var buf []rune
	i := 0

	for i < len(runes) {
		ch := runes[i]

		if ch == '$' && i+1 < len(runes) && (isIdentStart(runes[i+1]) || runes[i+1] == '*' || runes[i+1] == '!') {
			// Flush text buffer
			if len(buf) > 0 {
				parts = append(parts, &LiteralExpr{Type: TokString, Value: string(buf)})
				buf = nil
			}
			// Scan variable name
			i++ // skip $
			var varName []rune
			varName = append(varName, '$')
			// Twigil
			if i < len(runes) && (runes[i] == '*' || runes[i] == '!') {
				varName = append(varName, runes[i])
				i++
			}
			for i < len(runes) && isIdentContinue(runes[i]) {
				varName = append(varName, runes[i])
				i++
			}
			parts = append(parts, &VarExpr{Name: string(varName)})
			continue
		}

		if ch == '{' {
			// Flush text buffer
			if len(buf) > 0 {
				parts = append(parts, &LiteralExpr{Type: TokString, Value: string(buf)})
				buf = nil
			}
			// Collect expression text until '}'
			i++ // skip {
			var exprBuf []rune
			depth := 1
			for i < len(runes) && depth > 0 {
				if runes[i] == '{' {
					depth++
				} else if runes[i] == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				exprBuf = append(exprBuf, runes[i])
				i++
			}
			if i < len(runes) {
				i++ // skip closing }
			}
			// Parse the expression
			exprStr := string(exprBuf)
			lexer := NewLexer(exprStr)
			var tokens []Token
			for {
				tok := lexer.NextToken()
				tokens = append(tokens, tok)
				if tok.Type == TokEOF {
					break
				}
			}
			subParser := NewParser(tokens)
			expr, err := subParser.parseExpression(0)
			if err != nil {
				return nil, fmt.Errorf("interpolation error in \"{%s}\": %w", exprStr, err)
			}
			parts = append(parts, expr)
			continue
		}

		buf = append(buf, ch)
		i++
	}

	if len(buf) > 0 {
		parts = append(parts, &LiteralExpr{Type: TokString, Value: string(buf)})
	}

	if len(parts) == 0 {
		return &LiteralExpr{Type: TokString, Value: ""}, nil
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return &InterpStringExpr{Parts: parts}, nil
}

func isIdentStart(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
}

func isIdentContinue(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9')
}

func getPrecedence(tok Token) int {
	if tok.Type == TokIdent && (tok.Literal == "xor" || tok.Literal == "div" || tok.Literal == "mod") {
		return 1
	}
	switch tok.Type {
	case TokOr, TokDefinedOr:
		return 1
	case TokAnd:
		return 2
	case TokEqual, TokNotEqual, TokLess, TokLessEq, TokGreater, TokGreaterEq, TokEqStr, TokNeStr, TokLtStr, TokGtStr, TokAngleL, TokAngleR, TokElem, TokNotElem, TokDivisible, TokSmartMatch, TokRegexMatch, TokRegexNotMatch, TokMin, TokMax:
		return 4
	case TokConcat, TokRepeat, TokListRepeat, TokDotDot, TokIntersect, TokUnionOp, TokBitAnd, TokBitOr, TokBitXor, TokBitShiftL, TokBitShiftR:
		return 5
	case TokPlus, TokMinus:
		return 6
	case TokStar, TokSlash, TokPercent, TokDiv, TokMod:
		return 7
	case TokPower:
		return 8

	default:
		return 0
	}
}

func isComparisonOp(t TokenType) bool {
	return t == TokLess || t == TokLessEq || t == TokGreater || t == TokGreaterEq || t == TokEqual || t == TokNotEqual || t == TokAngleL || t == TokAngleR
}


func isComparisonOpStr(op string) bool {
	return op == "<" || op == "<=" || op == ">" || op == ">=" || op == "==" || op == "!="
}


func (p *Parser) isTypeToken(t TokenType) bool {
	return false
}

func (p *Parser) isBinaryOp(t Token) bool {
	return getPrecedence(t) > 0 || t.Type == TokAssign || t.Type == TokAddAssign || t.Type == TokSubAssign || t.Type == TokConcatAssign || t.Type == TokDefinedOrAssign
}


func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekNext() Token {
	if p.pos+1 >= len(p.tokens) {
		return Token{Type: TokEOF}
	}
	return p.tokens[p.pos+1]
}

func (p *Parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) consume(t TokenType) error {
	if p.peek().Type == t {
		p.advance()
		return nil
	}
	return fmt.Errorf("line %d: expected %v, got %s", p.peek().Line, t, p.peek().Literal)
}

func (p *Parser) match(t TokenType) bool {
	if p.peek().Type == t {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) isAtEnd() bool {
	return p.peek().Type == TokEOF
}

// parseSubsetDecl parses: subset Positive where { $_ > 0 } or subset Even of Int where { $_ %% 2 == 0 }
func (p *Parser) parseSubsetDecl() (Stmt, error) {
	p.advance() // consume 'subset'
	nameTok := p.advance()
	if nameTok.Type != TokIdent {
		return nil, fmt.Errorf("line %d: expected subset name, got %s", nameTok.Line, nameTok.Literal)
	}
	name := nameTok.Literal

	// Optional 'of Type'
	if p.peek().Type == TokIdent && p.peek().Literal == "of" {
		p.advance()
		p.advance() // skip base type
	}

	if p.peek().Type == TokWhere {
		p.advance()
	}

	var whereExpr Expr
	if p.peek().Type == TokLBrace {
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		whereExpr = &ClosureExpr{Body: body}
	} else {
		we, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		whereExpr = we
	}

	p.match(TokSemicolon)
	return &SubsetDeclStmt{Name: name, Where: whereExpr}, nil
}

// parseStructDecl parses: struct Point { int32 $x; int32 $y; } or union Payload { int64 $i; num64 $f; }
func (p *Parser) parseStructDecl() (Stmt, error) {
	kw := p.advance()
	isUnion := kw.Type == TokUnion
	nameTok := p.advance()
	structName := nameTok.Literal

	p.consume(TokLBrace)

	var fields []CStructField
	offset := 0
	maxAlign := 1

	for p.peek().Type != TokRBrace && !p.isAtEnd() {
		fieldType := "int64"
		if p.isTypeToken(p.peek().Type) || p.peek().Type == TokIdent {
			fieldType = p.advance().Literal
		}
		fieldVar := p.advance()
		fieldName := strings.TrimPrefix(fieldVar.Literal, "$")
		fieldName = strings.TrimPrefix(fieldName, "@")
		fieldName = strings.TrimPrefix(fieldName, "%")

		size, align := getCFieldSizeAndAlign(fieldType)
		if align > maxAlign {
			maxAlign = align
		}

		fieldOffset := 0
		if isUnion {
			fieldOffset = 0
		} else {
			// Natural alignment padding
			if align > 1 {
				rem := offset % align
				if rem != 0 {
					offset += (align - rem)
				}
			}
			fieldOffset = offset
			offset += size
		}

		fields = append(fields, CStructField{
			Name:   fieldName,
			Type:   fieldType,
			Offset: fieldOffset,
			Size:   size,
		})

		p.match(TokSemicolon)
	}

	p.consume(TokRBrace)
	p.match(TokSemicolon)

	totalSize := offset
	if isUnion {
		for _, f := range fields {
			if f.Size > totalSize {
				totalSize = f.Size
			}
		}
	} else {
		// Align total size to struct alignment
		if maxAlign > 1 {
			rem := totalSize % maxAlign
			if rem != 0 {
				totalSize += (maxAlign - rem)
			}
		}
	}

	fieldIndex := make(map[string]int, len(fields))
	for i, f := range fields {
		fieldIndex[f.Name] = i
	}

	return &CStructDeclStmt{
		Name:       structName,
		IsUnion:    isUnion,
		Fields:     fields,
		FieldIndex: fieldIndex,
		TotalSize:  totalSize,
		Alignment:  maxAlign,
	}, nil
}

func getCFieldSizeAndAlign(typeName string) (int, int) {
	switch typeName {
	case "int8", "uint8", "byte", "char", "bool", "Bool":
		return 1, 1
	case "int16", "uint16", "short", "WORD":
		return 2, 2
	case "int32", "uint32", "int", "uint", "long", "DWORD", "num32", "float32":
		return 4, 4
	case "int64", "uint64", "Int", "num64", "float64", "Num", "double", "ptr", "pointer", "Pointer", "OpaquePointer", "Str", "CStr":
		return 8, 8
	default:
		return 8, 8
	}
}

// parseAssert parses: assert <condition> [, <message>];
func (p *Parser) parseAssert() (Stmt, error) {
	p.advance() // skip 'assert'
	cond, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	var msg Expr
	if p.peek().Type == TokComma {
		p.advance()
		m, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		msg = m
	}
	p.match(TokSemicolon)
	return &AssertStmt{Condition: cond, Message: msg}, nil
}

// parseAdviceHook parses: before|after|around subName(params) { ... }
func (p *Parser) parseAdviceHook() (Stmt, error) {
	kindTok := p.advance()
	kind := kindTok.Literal // "before", "after", "around"
	nameTok := p.advance()
	targetName := nameTok.Literal

	var params []Param
	if p.peek().Type == TokLParen {
		p.advance()
		for p.peek().Type != TokRParen && !p.isAtEnd() {
			var paramType string
			if p.isTypeToken(p.peek().Type) || (p.peek().Type == TokIdent && (p.peekNext().Type == TokScalar || p.peekNext().Type == TokArray || p.peekNext().Type == TokHash || p.peekNext().Type == TokSubRef)) {
				paramType = p.advance().Literal
			}
			paramTok := p.advance()
			pName := paramTok.Literal
			params = append(params, Param{Name: pName, Type: paramType})
			if p.peek().Type == TokComma {
				p.advance()
			}
		}
		p.consume(TokRParen)
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &AdviceHookStmt{
		Kind:       kind,
		TargetName: targetName,
		Params:     params,
		Body:       body,
	}, nil
}

