package parity

func StripComments(src string) string {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); {
		ch := src[i]
		if ch == '\'' || ch == '"' || ch == '`' {
			end := skipLiteral(src, i)
			out = append(out, src[i:end]...)
			i = end
			continue
		}
		if ch == '/' && i+1 < len(src) {
			switch src[i+1] {
			case '/':
				i = skipLine(src, i)
				out = append(out, '\n')
				continue
			case '*':
				i = skipBlock(src, i)
				out = append(out, '\n')
				continue
			}
		}
		out = append(out, ch)
		i++
	}
	return string(out)
}

func skipLiteral(src string, start int) int {
	quote := src[start]
	i := start + 1
	for i < len(src) {
		if src[i] == '\\' {
			i += 2
			continue
		}
		if src[i] == quote {
			return i + 1
		}
		i++
	}
	return i
}

func skipLine(src string, at int) int {
	for at < len(src) && src[at] != '\n' {
		at++
	}
	return at
}

func skipBlock(src string, at int) int {
	at += 2
	for at+1 < len(src) {
		if src[at] == '*' && src[at+1] == '/' {
			return at + 2
		}
		at++
	}
	return len(src)
}
