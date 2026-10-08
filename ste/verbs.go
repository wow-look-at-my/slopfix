package ste

import "strings"

// The verb forms a simple-tense repair needs. rules/ste-verbs.xml carries a
// verb as the base form.

// verb is one line of the table.
type verb struct {
	base, past, participle string
}

var verbs = func() []verb {
	var out []verb
	for _, name := range []string{"verbs", "silent-e"} {
		for _, line := range steTable.List(name) {
			fields := strings.Fields(line)
			if len(fields) != 3 {
				continue
			}
			out = append(out, verb{base: fields[0], past: fields[1], participle: fields[2]})
		}
	}
	return out
}()

// verbBy answers the table entry a form belongs to. The form is matched
// against every column, so a caller can ask from the base, the past or the
// participle alike.
func verbBy(form string) (verb, bool) {
	form = strings.ToLower(form)
	for _, v := range verbs {
		if v.base == form || v.past == form || v.participle == form {
			return v, true
		}
	}
	return verb{}, false
}

// verbByGerund answers the table entry that writes form as its gerund.
func verbByGerund(form string) (verb, bool) {
	form = strings.ToLower(form)
	for _, v := range verbs {
		if v.base+"ing" == form {
			return v, true
		}
	}
	return verb{}, false
}

// pastFromParticiple answers the simple past of a participle.
func pastFromParticiple(form string) string {
	if v, ok := verbBy(form); ok && strings.EqualFold(v.participle, form) {
		return v.past
	}
	return form
}

// baseFromGerund answers the verb a gerund belongs to. A doubled consonant
// before -ing is dropped, and the table answers every form an ending hides.
func baseFromGerund(form string) string {
	if v, ok := verbByGerund(form); ok {
		return v.base
	}
	lower := strings.ToLower(form)
	if !strings.HasSuffix(lower, "ing") || len(lower) <= 4 {
		return form
	}
	stem := lower[:len(lower)-3]
	if doubled(stem) {
		return stem[:len(stem)-1]
	}
	return stem
}

// doubled reports a stem ending in a consonant written twice, as in "stopped"
// and "running".
func doubled(stem string) bool {
	if len(stem) < 2 || stem[len(stem)-1] != stem[len(stem)-2] {
		return false
	}
	return !strings.ContainsRune("aeiou", rune(stem[len(stem)-1]))
}

// presentOf writes the simple present of a base form for the person its
// subject names.
func presentOf(base string, plural bool) string {
	if plural {
		return base
	}
	lower := strings.ToLower(base)
	switch {
	case strings.HasSuffix(lower, "s"), strings.HasSuffix(lower, "x"),
		strings.HasSuffix(lower, "z"), strings.HasSuffix(lower, "ch"),
		strings.HasSuffix(lower, "sh"), strings.HasSuffix(lower, "o"):
		return base + "es"
	case strings.HasSuffix(lower, "y") && len(lower) > 1 && !strings.ContainsRune("aeiou", rune(lower[len(lower)-2])):
		return base[:len(base)-1] + "ies"
	}
	return base + "s"
}
