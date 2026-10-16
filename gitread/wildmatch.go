package gitread

// This file ports git's wildmatch.c, which gitignore and gitattributes
// patterns run through.

const (
	wmMatch = iota
	wmNoMatch
	wmAbortAll
	wmAbortToStarStar
)

// wildmatch reports whether text matches pattern.
func wildmatch(pattern, text string, pathname, fold bool) bool {
	return dowild(pattern, text, pathname, fold) == wmMatch
}

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func dowild(p, text string, pathname, fold bool) int {
	pi, ti := 0, 0
	for ; pi < len(p); pi++ {
		pch := p[pi]
		if ti >= len(text) && pch != '*' {
			return wmAbortAll
		}
		var tch byte
		if ti < len(text) {
			tch = text[ti]
		}
		if fold {
			tch, pch = lower(tch), lower(pch)
		}
		switch pch {
		case '\\':
			pi++
			if pi >= len(p) {
				return wmAbortAll
			}
			pch = p[pi]
			if fold {
				pch = lower(pch)
			}
			if tch != pch {
				return wmNoMatch
			}
			ti++
		case '?':
			if pathname && tch == '/' {
				return wmNoMatch
			}
			ti++
		case '*':
			if r, done := star(p, &pi, text, &ti, pathname, fold); done {
				return r
			}
		case '[':
			ok, next := matchClass(p, pi, tch, pathname, fold)
			if next < 0 {
				return wmAbortAll
			}
			if !ok {
				return wmNoMatch
			}
			pi = next
			ti++
		default:
			if tch != pch {
				return wmNoMatch
			}
			ti++
		}
	}
	if ti < len(text) {
		return wmNoMatch
	}
	return wmMatch
}

// star matches the wildcard at p[*pi]. It answers a result, or done false when
// the match goes on at the next pattern byte with text advanced.
func star(p string, pi *int, text string, ti *int, pathname, fold bool) (int, bool) {
	i := *pi + 1
	matchSlash := !pathname
	if i < len(p) && p[i] == '*' {
		prev := *pi - 1
		for i < len(p) && p[i] == '*' {
			i++
		}
		if !pathname {
			matchSlash = true
		} else if (prev < 0 || p[prev] == '/') && (i == len(p) || p[i] == '/' || (p[i] == '\\' && i+1 < len(p) && p[i+1] == '/')) {
			if i < len(p) && p[i] == '/' && dowild(p[i+1:], text[*ti:], pathname, fold) == wmMatch {
				return wmMatch, true
			}
			matchSlash = true
		} else {
			matchSlash = false
		}
	}
	if i == len(p) {
		// A trailing "*" matches only up to the next slash. A trailing "**" matches the rest.
		if !matchSlash {
			for j := *ti; j < len(text); j++ {
				if text[j] == '/' {
					return wmAbortToStarStar, true
				}
			}
		}
		return wmMatch, true
	}
	if !matchSlash && p[i] == '/' {
		for j := *ti; j < len(text); j++ {
			if text[j] == '/' {
				*ti = j
				*pi = i - 1
				return 0, false
			}
		}
		return wmAbortAll, true
	}
	for t := *ti; t < len(text); t++ {
		m := dowild(p[i:], text[t:], pathname, fold)
		if m != wmNoMatch {
			if !matchSlash || m != wmAbortToStarStar {
				return m, true
			}
		} else if !matchSlash && text[t] == '/' {
			return wmAbortToStarStar, true
		}
	}
	return wmAbortAll, true
}

// matchClass matches a bracket expression that starts at p[open].
func matchClass(p string, open int, tch byte, pathname, fold bool) (bool, int) {
	i := open + 1
	if i >= len(p) {
		return false, -1
	}
	negate := p[i] == '!' || p[i] == '^'
	if negate {
		i++
	}
	matched := false
	var prev byte
	first := true
	for ; i < len(p); i++ {
		c := p[i]
		if c == ']' && !first {
			if pathname && tch == '/' {
				return false, i
			}
			return matched != negate, i
		}
		first = false
		switch {
		case c == '\\':
			i++
			if i >= len(p) {
				return false, -1
			}
			c = p[i]
			if eq(c, tch, fold) {
				matched = true
			}
		case c == '-' && prev != 0 && i+1 < len(p) && p[i+1] != ']':
			i++
			hi := p[i]
			if hi == '\\' {
				i++
				if i >= len(p) {
					return false, -1
				}
				hi = p[i]
			}
			if prev <= tch && tch <= hi {
				matched = true
			}
			if fold {
				lt := lower(tch)
				if lower(prev) <= lt && lt <= lower(hi) {
					matched = true
				}
			}
			c = 0
		case c == '[' && i+1 < len(p) && p[i+1] == ':':
			end := indexFrom(p, i+2, ":]")
			if end < 0 {
				if eq(c, tch, fold) {
					matched = true
				}
				break
			}
			if inClass(p[i+2:end], tch, fold) {
				matched = true
			}
			i = end + 1
			c = 0
		default:
			if eq(c, tch, fold) {
				matched = true
			}
		}
		prev = c
	}
	return false, -1
}

func eq(a, b byte, fold bool) bool {
	return a == b || fold && lower(a) == lower(b)
}

func indexFrom(s string, from int, sub string) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// inClass reports whether c is in a named POSIX class.
func inClass(name string, c byte, fold bool) bool {
	isUpper := c >= 'A' && c <= 'Z'
	isLower := c >= 'a' && c <= 'z'
	isDigit := c >= '0' && c <= '9'
	isAlpha := isUpper || isLower
	switch name {
	case "alnum":
		return isAlpha || isDigit
	case "alpha":
		return isAlpha
	case "blank":
		return c == ' ' || c == '\t'
	case "cntrl":
		return c < 0x20 || c == 0x7f
	case "digit":
		return isDigit
	case "graph":
		return c > 0x20 && c < 0x7f
	case "lower":
		return isLower || fold && isUpper
	case "print":
		return c >= 0x20 && c < 0x7f
	case "punct":
		return c > 0x20 && c < 0x7f && !isAlpha && !isDigit
	case "space":
		return c == ' ' || c >= '\t' && c <= '\r'
	case "upper":
		return isUpper || fold && isLower
	case "xdigit":
		return isDigit || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
	}
	return false
}
