package gcre

import "strings"

func applyRxQuant(p rx, q byte) rx {
	if q == ' ' || p == nil {
		return p
	}
	switch t := p.(type) {
	case *rxSub:
		t.quant = q
		return t
	case *rxCC:
		t.quant = q
		return t
	case *rxGrp:
		t.quant = q
		return t
	default:
		return &rxGrp{inner: p, quant: q}
	}
}

func unescapeRune(s string) rune {
	if s == "" {
		return 0
	}
	switch s[0] {
	case 'n', 'N':
		return '\n'
	case 't', 'T':
		return '\t'
	case 'r', 'R':
		return '\r'
	default:
		return []rune(s)[0]
	}
}

func toLowerASCII(s string) string { return strings.ToLower(s) }
