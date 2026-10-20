package xmlcheck

// Pattern is a table's match with the entry's case rule compiled into it, for a
// caller that precompiles the source rather than compiling it here.
func Pattern(match string, sensitive bool) string {
	if !sensitive {
		return "(?i)" + match
	}
	return match
}
