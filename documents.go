package slopfix

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/jsonc"
	jsonvalidator "github.com/wow-look-at-my/json-validator/validator"
	"github.com/wow-look-at-my/slopfix/commentfix"
	xmlvalidator "github.com/wow-look-at-my/xml-validator/validator"
)

// IDJSON is a JSON file that does not parse, or that breaks the schema its $schema names.
const IDJSON = "repo/json"

// IDXML is an XML file the strict org validator refuses, that names no schema, or that breaks the schema it names.
const IDXML = "repo/xml"

// The document failures a rewrite answers.
const (
	ruleJSONUnparseable     = "this JSON does not parse"
	ruleNegativeFixturePass = "this negative fixture passes the schema it names, so it tests nothing"
	ruleJSONSchemaMissing   = "the schema its $schema names does not load"
)

// closeBrackets answers a document with every bracket it leaves open closed,
// which is the JSON failure a rewrite decides without reading the author's
// intent. A bracket inside a string is text, so the scan skips strings.
func closeBrackets(content []byte) []byte {
	var stack []byte
	inString, escaped := false, false
	for _, b := range content {
		if inString {
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, b)
		case '}':
			stack = closeIn(stack, '{')
		case ']':
			stack = closeIn(stack, '[')
		}
	}
	out := bytes.TrimRight(content, "\n")
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			out = append(out, '}')
			continue
		}
		out = append(out, ']')
	}
	return append(out, '\n')
}

// dropTrailingCommas answers a document with each comma that stands before a
// closing bracket taken out. A comma inside a string is text.
func dropTrailingCommas(content []byte) []byte {
	out := make([]byte, 0, len(content))
	inString, escaped := false, false
	for i, b := range content {
		if inString {
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inString = false
			}
			out = append(out, b)
			continue
		}
		if b == '"' {
			inString = true
		}
		if b == ',' {
			next := bytes.TrimLeft(content[i+1:], " \t\r\n")
			if len(next) > 0 && (next[0] == '}' || next[0] == ']') {
				continue
			}
		}
		out = append(out, b)
	}
	return out
}

// schemaMember matches a top-level "$schema" member and the comma after it.
var schemaMember = regexp.MustCompile(`"\$schema"\s*:\s*"(?:[^"\\]|\\.)*"\s*,?`)

// dropSchemaMember answers a document without the "$schema" member that names
// a schema nothing can load. The document is then held to no schema.
func dropSchemaMember(content []byte) []byte {
	loc := schemaMember.FindIndex(content)
	if loc == nil {
		return content
	}
	return append(append([]byte{}, content[:loc[0]]...), content[loc[1]:]...)
}

// closeIn answers the stack with the bracket at the top of the matching kind
// taken off.
func closeIn(stack []byte, open byte) []byte {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == open {
			return stack[:i]
		}
	}
	return stack
}

// renamedWithoutMarker answers the name a *.invalid.xml fixture takes when
// the repair drops the marker.
func renamedWithoutMarker(rel string) string {
	return strings.TrimSuffix(rel, ".invalid.xml") + ".xml"
}

// isJSON reports whether a file holds JSON, comments allowed.
func isJSON(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".json" || ext == ".jsonc"
}

func isXML(path string) bool { return filepath.Ext(path) == ".xml" }

// schemas compiles each JSON schema once per walk, because many files name the same one.
type schemas map[string]struct {
	v   *jsonvalidator.Validator
	err error
}

func (s schemas) get(ref string) (*jsonvalidator.Validator, error) {
	if got, ok := s[ref]; ok {
		return got.v, got.err
	}
	v, err := jsonvalidator.New(jsonvalidator.Options{SchemaPath: ref})
	s[ref] = struct {
		v   *jsonvalidator.Validator
		err error
	}{v, err}
	return v, err
}

// documents reports each JSON and XML file under root that fails its checks.
func documents(root string, keeps func(string) bool) ([]TreeFinding, error) {
	var out []TreeFinding
	compiled := schemas{}
	downloads := fetched{}
	for _, path := range commentfix.TreeFilesMatching(root, func(path string) bool {
		return (keeps(IDJSON) && isJSON(path)) || (keeps(IDXML) && isXML(path))
	}) {
		content, err := os.ReadFile(path)
		if err != nil {
			return out, err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return out, err
		}
		var finding *TreeFinding
		if isJSON(path) {
			finding = checkJSON(path, rel, content, compiled)
		} else {
			finding = checkXML(path, rel, content, downloads)
		}
		if finding != nil {
			out = append(out, *finding)
		}
	}
	return out, nil
}

func remote(ref string) bool { return strings.Contains(ref, "://") }

// fetched holds each remote XSD a walk downloads, because many files name the same one.
type fetched map[string]struct {
	body []byte
	err  error
}

func (f fetched) get(url string) ([]byte, error) {
	if got, ok := f[url]; ok {
		return got.body, got.err
	}
	body, err := fetch(url)
	f[url] = struct {
		body []byte
		err  error
	}{body, err}
	return body, err
}

// fetch downloads a schema. Any status but OK is an error, never an empty schema.
func fetch(url string) ([]byte, error) {
	resp, err := schemaClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

var schemaClient = &http.Client{Timeout: 30 * time.Second}

// anyJSON accepts every document, so json-validator alone decides whether a file parses.
var anyJSON = func() *jsonvalidator.Validator {
	v, err := jsonvalidator.NewFromBytes("embedded:any.schema.json", []byte(`{}`), jsonvalidator.Options{})
	if err != nil {
		panic(err)
	}
	return v
}()

// checkJSON parses a JSON file, then holds it to the schema its $schema names.
// A relative $schema is a path from the file's directory. A URL is fetched, and
// a schema that does not load is a finding, never a pass.
func checkJSON(path, rel string, content []byte, compiled schemas) *TreeFinding {
	if result := anyJSON.ValidateBytes(content, rel); result.Err != nil {
		f := repoFinding(rel, IDJSON, "this JSON does not parse", result.Err.Error())
		var syntax *json.SyntaxError
		if errors.As(result.Err, &syntax) {
			f.Line = lineAt(content, syntax.Offset)
		}
		return &f
	}
	// The validator above said the file parses. This reads its $schema the same way.
	var doc any
	if err := json.Unmarshal(jsonc.ToJSON(content), &doc); err != nil {
		f := repoFinding(rel, IDJSON, "this JSON does not parse", err.Error())
		return &f
	}
	object, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	ref, ok := object["$schema"].(string)
	if !ok {
		return nil
	}
	if !remote(ref) {
		ref = filepath.Join(filepath.Dir(path), filepath.FromSlash(ref))
	}
	v, err := compiled.get(ref)
	if err != nil {
		f := repoFinding(rel, IDJSON, ruleJSONSchemaMissing, err.Error())
		return &f
	}
	result := v.ValidateBytes(content, rel)
	if result.Err != nil {
		f := repoFinding(rel, IDJSON, "this JSON does not parse", result.Err.Error())
		return &f
	}
	if !result.Valid {
		f := repoFinding(rel, IDJSON, "this JSON breaks the schema its $schema names", result.Detail())
		return &f
	}
	return nil
}

const xsiNamespace = "http://www.w3.org/2001/XMLSchema-instance"

// schemaHint reads the schema the root element names from the parsed tree. A
// root with no namespace names it in xsi:noNamespaceSchemaLocation. A root in a
// namespace names it in the xsi:schemaLocation pair for that namespace.
func schemaHint(root *xmlvalidator.Element) (string, error) {
	for _, a := range root.Attrs {
		if a.Namespace != xsiNamespace {
			continue
		}
		if root.Namespace == "" && a.Local == "noNamespaceSchemaLocation" {
			return strings.TrimSpace(a.Value), nil
		}
		if root.Namespace == "" || a.Local != "schemaLocation" {
			continue
		}
		pairs := strings.Fields(a.Value)
		if len(pairs)%2 != 0 {
			return "", fmt.Errorf("xsi:schemaLocation holds %d values, and it must hold namespace and location pairs", len(pairs))
		}
		for i := 0; i < len(pairs); i += 2 {
			if pairs[i] == root.Namespace {
				return pairs[i+1], nil
			}
		}
	}
	return "", nil
}

// checkXML holds an XML file to the strict org validator and to the schema it
// names. A document that names no schema is a finding, because only its syntax
// gets a check. A remote schema is fetched, and so is each import it names.
func checkXML(path, rel string, content []byte, downloads fetched) *TreeFinding {
	if err := xmlvalidator.Validate(bytes.NewReader(content)); err != nil {
		return xmlFinding(rel, "this XML is not well-formed", err)
	}
	doc, err := xmlvalidator.ParseTree(bytes.NewReader(content))
	if err != nil {
		return xmlFinding(rel, "this XML is not well-formed", err)
	}
	location, err := schemaHint(doc.Root)
	if err != nil {
		f := repoFinding(rel, IDXML, "the schema hint on this XML does not parse", err.Error())
		return &f
	}
	if location == "" {
		f := repoFinding(rel, IDXML, "this XML names no schema, so nothing checks its structure",
			"Name its XSD with xsi:noNamespaceSchemaLocation, or with an xsi:schemaLocation pair for the root's namespace.")
		return &f
	}
	schema, err := loadSchema(path, location, downloads)
	if err != nil {
		f := repoFinding(rel, IDXML, "the schema this XML names does not load", err.Error())
		return &f
	}
	verr := xmlvalidator.ValidateSchema(doc, schema)
	if !negativeFixture(rel) {
		if verr != nil {
			return xmlFinding(rel, "this XML breaks the schema it names", verr)
		}
		return nil
	}
	if verr == nil {
		f := repoFinding(rel, IDXML, "this negative fixture passes the schema it names, so it tests nothing",
			"A file named *.invalid.xml must break its schema. Break it on the constraint it exists to test, or rename it.")
		return &f
	}
	return nil
}

// negativeFixture reports a document named to fail its schema. Such a file is
// held to the opposite test: the schema must reject it.
func negativeFixture(rel string) bool {
	return strings.HasSuffix(filepath.Base(rel), ".invalid.xml")
}

// loadSchema parses the schema a document names, with each import it names. A
// remote location is fetched. A local one is a path from the document.
func loadSchema(path, location string, downloads fetched) (*xmlvalidator.Schema, error) {
	if !remote(location) {
		location = filepath.Join(filepath.Dir(path), filepath.FromSlash(location))
		body, err := os.ReadFile(location)
		if err != nil {
			return nil, err
		}
		return parseSchema(body, xmlvalidator.FileSchemaResolver(filepath.Dir(location)))
	}
	base, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("the location is not a URL: %w", err)
	}
	body, err := downloads.get(location)
	if err != nil {
		return nil, err
	}
	return parseSchema(body, func(_, hint string) ([]byte, error) {
		next, err := base.Parse(hint)
		if err != nil {
			return nil, err
		}
		return downloads.get(next.String())
	})
}

func parseSchema(body []byte, imports xmlvalidator.SchemaResolver) (*xmlvalidator.Schema, error) {
	doc, err := xmlvalidator.ParseTree(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return xmlvalidator.ParseSchemaWithResolver(doc, imports)
}

func xmlFinding(rel, rule string, err error) *TreeFinding {
	f := repoFinding(rel, IDXML, rule, err.Error())
	var located *xmlvalidator.Error
	if errors.As(err, &located) && located.Line > 0 {
		f.Line = located.Line
	}
	return &f
}

// lineAt is the line that holds a byte offset.
func lineAt(content []byte, offset int64) int {
	if offset > int64(len(content)) {
		offset = int64(len(content))
	}
	return bytes.Count(content[:offset], []byte("\n")) + 1
}
