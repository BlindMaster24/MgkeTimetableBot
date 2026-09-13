package parity

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

type Literal struct {
	Value string
	Start int
	End   int
}

func StringLiterals(src string) []string {
	literals := StringLiteralsAt(src)
	out := make([]string, 0, len(literals))
	for _, literal := range literals {
		out = append(out, literal.Value)
	}
	return out
}

func StringLiteralsAt(src string) []Literal {
	var out []Literal
	for i := 0; i < len(src); {
		ch := src[i]
		if ch != '\'' && ch != '"' && ch != '`' {
			i++
			continue
		}
		value, next := readLiteral(src, i)
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, Literal{Value: value, Start: i, End: next})
		}
		i = next
	}
	return out
}

func readLiteral(src string, start int) (string, int) {
	quote := src[start]
	var b strings.Builder
	i := start + 1
	for i < len(src) {
		ch := src[i]
		if ch == '\\' && i+1 < len(src) {
			decoded, next := decodeEscape(src, i)
			b.WriteString(decoded)
			i = next
			continue
		}
		if ch == quote {
			return b.String(), i + 1
		}
		if ch == '\n' && quote != '`' {
			return b.String(), i
		}
		b.WriteByte(ch)
		i++
	}
	return b.String(), i
}

type surrogatePair struct {
	r    rune
	next int
}

func combineSurrogates(src string, at int, high rune) (surrogatePair, bool) {
	if high < 0xD800 || high > 0xDBFF || at+6 > len(src) || src[at] != '\\' || src[at+1] != 'u' {
		return surrogatePair{}, false
	}
	low, err := strconv.ParseInt(src[at+2:at+6], 16, 32)
	if err != nil {
		return surrogatePair{}, false
	}
	lr := rune(low)
	if lr < 0xDC00 || lr > 0xDFFF {
		return surrogatePair{}, false
	}
	return surrogatePair{r: 0x10000 + (high-0xD800)<<10 + (lr - 0xDC00), next: at + 6}, true
}

func decodeEscape(src string, at int) (string, int) {
	if at+1 >= len(src) {
		return "\\", at + 1
	}
	switch src[at+1] {
	case 'n':
		return "\n", at + 2
	case 't':
		return "\t", at + 2
	case 'r':
		return "\r", at + 2
	case 'b':
		return "\b", at + 2
	case 'f':
		return "\f", at + 2
	case 'v':
		return "\v", at + 2
	case '0':
		return "\x00", at + 2
	case 'x':
		if at+3 < len(src) {
			if v, err := strconv.ParseInt(src[at+2:at+4], 16, 32); err == nil {
				return string(rune(v)), at + 4
			}
		}
		return "x", at + 2
	case 'u':
		if at+2 < len(src) && src[at+2] == '{' {
			if end := strings.IndexByte(src[at+3:], '}'); end >= 0 {
				hex := src[at+3 : at+3+end]
				if v, err := strconv.ParseInt(hex, 16, 32); err == nil {
					return string(rune(v)), at + 4 + end
				}
			}
			return "u", at + 2
		}
		if at+5 < len(src) {
			if v, err := strconv.ParseInt(src[at+2:at+6], 16, 32); err == nil {
				if high, ok := combineSurrogates(src, at+6, rune(v)); ok {
					return string(high.r), high.next
				}
				return string(rune(v)), at + 6
			}
		}
		return "u", at + 2
	default:
		r, size := utf8.DecodeRuneInString(src[at+1:])
		return string(r), at + 1 + size
	}
}
