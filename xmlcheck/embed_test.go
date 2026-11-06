package xmlcheck_test

import _ "embed"

// The schema. The tables the tests read live beside them rather than in a Go
// string, so each is the file a table would be.
var (
	//go:embed testdata/miniature.xsd
	miniatureSchema []byte

	//go:embed testdata/a-sound-table.xml
	soundTable string

	//go:embed testdata/an-unknown-element.xml
	unknownElement string

	//go:embed testdata/two-results-for-one-input.xml
	twoResultsForOneInput string

	//go:embed testdata/one-result-in-two-places.xml
	oneResultInTwoPlaces string

	//go:embed testdata/an-xml-11-table.xml
	xml11Table string
)
