package gcre

import (
	"strconv"
	"strings"
)

type quantSpec struct {
	present  bool
	min      int
	max      int // -1 means unbounded (*)
	frugal   bool
	sep      rx
	sepTrail bool // true for %%, false for %
}

func wrapQuant(p rx, q quantSpec) rx {
	if p == nil {
		return &rxNoop{}
	}
	if !q.present && q.sep == nil {
		return p
	}
	if !q.present && q.sep != nil {
		q.present = true
		q.min = 1
		q.max = -1
	}
	return &rxQuant{inner: p, q: q}
}

func unescapeSq(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '\\' || next == '\'' {
				b.WriteRune(next)
				i++
				continue
			}
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

func unescapeDq(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\', '"', '\'', '$', '{', '}':
				b.WriteRune(next)
			case 'x':
				if i+2 < len(runes) && runes[i+2] == '[' {
					end := i + 3
					for end < len(runes) && runes[end] != ']' {
						end++
					}
					if end < len(runes) {
						b.WriteString(decodeHexBracket(string(runes[i+3 : end])))
						i = end
						continue
					}
				}
				b.WriteRune(next)
			default:
				b.WriteRune(next)
			}
			i++
			continue
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

func decodeHexBracket(inner string) string {
	var b strings.Builder
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if v, err := strconv.ParseInt(part, 16, 32); err == nil {
			b.WriteRune(rune(v))
		}
	}
	return b.String()
}

func unescapeRune(s string) rune {
	if s == "" {
		return 0
	}
	switch s[0] {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case 'e':
		return 0x1B
	case 'f':
		return '\f'
	default:
		return []rune(s)[0]
	}
}

func isPureDeclarative(p rx) bool {
	if p == nil {
		return true
	}
	switch t := p.(type) {
	case *rxNoop, *rxLit, *rxAny, *rxCharClass, *rxAnchor:
		return true
	case *rxGrp:
		return isPureDeclarative(t.inner)
	case *rxCap:
		return isPureDeclarative(t.inner)
	case *rxAlias:
		return isPureDeclarative(t.inner)
	case *rxQuant:
		return isPureDeclarative(t.inner) && isPureDeclarative(t.q.sep)
	case *rxSeq:
		for _, e := range t.elts {
			if !isPureDeclarative(e) {
				return false
			}
		}
		return true
	case *rxAlt:
		for _, a := range t.alts {
			if !isPureDeclarative(a) {
				return false
			}
		}
		return true
	case *rxLTMAlt:
		for _, a := range t.alts {
			if !isPureDeclarative(a) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func forceLTM(p rx) rx {
	if p == nil {
		return nil
	}
	switch t := p.(type) {
	case *rxAlt:
		if !t.sequential {
			alts := make([]rx, len(t.alts))
			for i, a := range t.alts {
				alts[i] = forceLTM(a)
			}
			return &rxLTMAlt{alts: alts}
		}
		for i, a := range t.alts {
			t.alts[i] = forceLTM(a)
		}
		return t
	case *rxLTMAlt:
		for i, a := range t.alts {
			t.alts[i] = forceLTM(a)
		}
		return t
	case *rxSeq:
		for i, e := range t.elts {
			t.elts[i] = forceLTM(e)
		}
		return t
	case *rxGrp:
		t.inner = forceLTM(t.inner)
		return t
	case *rxCap:
		t.inner = forceLTM(t.inner)
		return t
	case *rxQuant:
		t.inner = forceLTM(t.inner)
		return t
	default:
		return p
	}
}
