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
		f := repoFinding(rel, IDJSON, "the schema its $schema names does not load", err.Error())
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

var (
	noNamespaceLocation = regexp.MustCompile(`noNamespaceSchemaLocation\s*=\s*["']([^"']+)["']`)
	namespacedLocation  = regexp.MustCompile(`schemaLocation\s*=\s*["']\s*[^\s"']+\s+([^\s"']+)[^"']*["']`)
)

// checkXML holds an XML file to the strict org validator and to the schema it
// names. A document that names no schema is a finding, because only its syntax
// gets a check. A remote schema is fetched, and so is each import it names.
func checkXML(path, rel string, content []byte, downloads fetched) *TreeFinding {
	if err := xmlvalidator.Validate(bytes.NewReader(content)); err != nil {
		return xmlFinding(rel, "this XML is not well-formed", err)
	}
	match := noNamespaceLocation.FindSubmatch(content)
	if match == nil {
		match = namespacedLocation.FindSubmatch(content)
	}
	if match == nil {
		f := repoFinding(rel, IDXML, "this XML names no schema, so nothing checks its structure",
			"Name its XSD with xsi:noNamespaceSchemaLocation.")
		return &f
	}
	location := string(match[1])
	if remote(location) {
		base, err := url.Parse(location)
		if err != nil {
			f := repoFinding(rel, IDXML, "the schema this XML names is not a URL", err.Error())
			return &f
		}
		schema, err := downloads.get(location)
		if err != nil {
			f := repoFinding(rel, IDXML, "the schema this XML names does not load", err.Error())
			return &f
		}
		imports := func(_, hint string) ([]byte, error) {
			next, err := base.Parse(hint)
			if err != nil {
				return nil, err
			}
			return downloads.get(next.String())
		}
		if err := xmlvalidator.ValidateWithSchemaResolver(content, schema, imports); err != nil {
			return xmlFinding(rel, "this XML breaks the schema it names", err)
		}
		return nil
	}
	location = filepath.Join(filepath.Dir(path), filepath.FromSlash(location))
	if err := xmlvalidator.ValidateWithSchemaFile(path, location); err != nil {
		return xmlFinding(rel, "this XML breaks the schema it names, or the schema does not load", err)
	}
	return nil
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
