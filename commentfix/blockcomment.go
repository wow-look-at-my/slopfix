package commentfix

import "strings"

// blockShape is how a /* */ comment is laid out, so a repair writes it back
// the same way.
type blockShape struct {
	indent string
	// opener is "/*" or "/**".
	opener string
	// starred is true when each continuation line opens with a star.
	starred bool
	// stars is the run of stars each continuation line opens with, "*" or "**".
	stars string
}

// starRun answers the run of stars every continuation line opens with, and
// false when a line opens with none.
func starRun(text []string) (string, bool) {
	run := ""
	for _, line := range text {
		body := strings.TrimLeft(line, " \t")
		stars := body[:len(body)-len(strings.TrimLeft(body, "*"))]
		if body == "*/" {
			continue
		}
		if stars == "" {
			return "", false
		}
		if run == "" || len(stars) < len(run) {
			run = stars
		}
	}
	return run, run != ""
}

// readBlock reports a run that is a single /* */ comment, closed on its last
// line, and answers its prose lines. A layout row keeps its extra indent.
func readBlock(text []string) (blockShape, []string, bool) {
	if len(text) == 0 {
		return blockShape{}, nil, false
	}
	first := strings.TrimLeft(text[0], " \t")
	last := strings.TrimSpace(text[len(text)-1])
	joined := strings.Join(text, "\n")
	if !strings.HasPrefix(first, "/*") || !strings.HasSuffix(last, "*/") || strings.Count(joined, "*/") != 1 || strings.Count(joined, "/*") != 1 {
		return blockShape{}, nil, false
	}
	shape := blockShape{indent: text[0][:len(text[0])-len(first)], opener: "/*"}
	if strings.HasPrefix(first, "/**") && !strings.HasPrefix(first, "/**/") {
		shape.opener = "/**"
	}
	if len(text) > 1 {
		shape.stars, shape.starred = starRun(text[1:])
	}
	// A continuation row past this column is laid out by hand.
	column := len(shape.indent) + len(shape.opener) + 1
	var prose []string
	for i, line := range text {
		line = strings.TrimRight(line, " \t")
		if i == len(text)-1 {
			line = strings.TrimRight(strings.TrimSuffix(line, "*/"), " \t")
		}
		switch {
		case i == 0:
			line = strings.TrimSpace(strings.TrimPrefix(first, shape.opener))
			if i == len(text)-1 {
				line = strings.TrimSpace(strings.TrimSuffix(line, "*/"))
			}
		case shape.starred:
			rest := strings.TrimPrefix(strings.TrimLeft(line, " \t"), shape.stars)
			line = strings.TrimPrefix(rest, " ")
		default:
			body := strings.TrimLeft(line, " \t")
			pad := len(line) - len(body)
			if pad > column && !strings.ContainsRune(line[:pad], '\t') {
				body = line[column:]
			}
			line = body
		}
		prose = append(prose, line)
	}
	for len(prose) > 0 && strings.TrimSpace(prose[0]) == "" {
		prose = prose[1:]
	}
	for len(prose) > 0 && strings.TrimSpace(prose[len(prose)-1]) == "" {
		prose = prose[:len(prose)-1]
	}
	if len(prose) == 0 {
		return blockShape{}, nil, false
	}
	return shape, prose, true
}

// asLines writes prose as line comments, the shape the trim reads.
func asLines(indent string, prose []string) []string {
	out := make([]string, 0, len(prose))
	for _, line := range prose {
		if strings.TrimSpace(line) == "" {
			out = append(out, indent+"//")
			continue
		}
		out = append(out, indent+"// "+line)
	}
	return out
}

// fromLines reads prose back out of line comments that asLines wrote.
func fromLines(text []string) []string {
	out := make([]string, 0, len(text))
	for _, line := range text {
		rest := strings.TrimPrefix(strings.TrimLeft(line, " \t"), "//")
		out = append(out, strings.TrimPrefix(rest, " "))
	}
	return out
}

// render writes prose as a /* */ comment in the given shape. No prose renders
// no comment.
func (s blockShape) render(prose []string) []string {
	if len(prose) == 0 {
		return nil
	}
	if len(prose) == 1 {
		return []string{s.indent + s.opener + " " + prose[0] + " */"}
	}
	lead := s.indent + strings.Repeat(" ", len(s.opener)+1)
	if s.starred {
		lead = s.indent + " * "
		if len(s.stars) > 1 {
			lead = s.indent + s.stars + " "
		}
	}
	out := []string{s.indent + s.opener + " " + prose[0]}
	for _, line := range prose[1:] {
		out = append(out, strings.TrimRight(lead+line, " \t"))
	}
	out[len(out)-1] += " */"
	return out
}

// repairBlockComment fits a /* */ comment to its budget. The trim runs on the
// comment as line comments.
func repairBlockComment(b block) ([]string, bool) {
	shape, prose, ok := readBlock(b.text)
	if !ok {
		return nil, false
	}
	lines := asLines(shape.indent, prose)
	kept := fromLines(trim(block{start: b.start, end: b.end, codeLines: b.codeLines, codeChars: b.codeChars, text: lines, exact: b.exact}))
	if out := shape.render(kept); len(out) > 0 && fitsCode(out, b) {
		return out, true
	}
	// No prose survives: the comment is a banner, and it goes.
	if len(kept) == 0 {
		return nil, true
	}
	// A cut between words leaves a fragment, so the comment stays for a rewrite by hand.
	return nil, false
}
