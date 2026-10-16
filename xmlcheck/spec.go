package xmlcheck

// A Key names an entry among its siblings. Entries that produce the same key
// are one entry written twice.
type Key struct {
	// Attrs are the attributes whose values form the key, compared with the entry's case rule.
	Attrs []string
	// Exact are attributes compared exactly whatever the case rule, such as the surface an entry applies to.
	Exact []string
	// Split reads the attribute as a comma-separated list, and every member is a key of its own.
	Split bool
}

// An Elem states what one element may carry.
type Elem struct {
	// Attrs may be present.
	Attrs []string
	// Required attributes are present and not blank.
	Required []string
	// Present attributes exist, and may hold the empty string.
	Present []string
	// Any is a set of attributes of which at least one is not blank.
	Any []string
	// Children are the element names allowed inside.
	Children []string
	// Text lets the element hold text of its own.
	Text bool
	// Case lets the element carry case=, which is how an entry changes the case rule it inherits.
	Case bool
	// Keys name the element among its siblings.
	Keys []Key
	// Enum limits an attribute to a set of values.
	Enum map[string][]string
	// Lists names the Enum attributes that hold several space-separated values.
	Lists []string
	// TestAttr and ExpectAttr name an inline test the entry carries on its own attributes.
	TestAttr, ExpectAttr string
}

// A TestDef says which element is a worked example and how it reads.
type TestDef struct {
	// Elem is the element name.
	Elem string
	// In is the attribute holding the input.
	In string
	// Out are the attributes that make up the expected result.
	Out []string
	// Body adds the element's text to the expected result.
	Body bool
	// EmptyOut makes an empty expectation a value that can contradict another test.
	EmptyOut bool
}

// A Shadow orders the entries of one element, applied first.
type Shadow struct {
	Elem string
	Attr string
}

// A Spec is everything a table file is held to.
type Spec struct {
	// Root is the document element.
	Root string
	// Elems is the vocabulary, the root included.
	Elems map[string]Elem
	// Test is the worked example element.
	Test TestDef
	// Sensitive makes the table match as typed: the root must say case="sensitive" and no entry may say otherwise.
	Sensitive bool
	// Shadows are the ordered phrase lists.
	Shadows []Shadow
	// Entry checks one element the way only its consumer can, and answers a problem for each defect.
	Entry func(n *Node, sensitive bool) []string
	// Fires answers whether an entry matches an input, for each entry that carries tests.
	Fires func(n *Node, sensitive bool, in string) bool
	// FiresAny answers whether some entry of the table matches an input.
	FiresAny func(roots []*Node, in string) bool
}

// A File is a table file and its name.
type File struct {
	Name string
	Raw  []byte
}

// A Problem is a defect, with where it sits.
type Problem struct {
	File string
	Line int
	Msg  string
}
