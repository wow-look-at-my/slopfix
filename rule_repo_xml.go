package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// repo/xml: an XML file the strict org validator refuses, that names no schema,
// or that breaks the schema it names. A file named *.invalid.xml is held to the
// opposite test, and one that passes its schema tests nothing.
//
// The repair drops the *.invalid marker, which is the rename the finding itself
// names: the file then stands as an ordinary document. It passes the schema it
// declares.
func init() {
	RegisterRule(RuleSpec{
		ID:       IDXML,
		Category: RuleRepo,
		Detect:   detectXML,
		Autofix:  autofixXML,
		Cases: []RuleCase{{Name: IDXML, Files: map[string]string{
			"thing.invalid.xml": `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="ok.xsd">hello</root>`,
			"ok.xsd":            `<?xml version="1.0"?><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`,
		}}},
	})
}

// detectXML answers every XML document the rule reports under root.
func detectXML(c RuleCase) []ste.Finding { return treeFindings(c, IDXML) }

// autofixXML drops the *.invalid marker from a fixture that passes its schema.
func autofixXML(c RuleCase) RuleCase { return treeAutofix(c, IDXML) }
