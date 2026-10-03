package slopfix

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
			finding = checkXML(path, rel, content)
		}
		if finding != nil {
			out = append(out, *finding)
		}
	}
	return out, nil
}

// checkJSON parses a JSON file, then holds it to the schema its $schema names.
// A relative $schema is a path from the file's directory.
func checkJSON(path, rel string, content []byte, compiled schemas) *TreeFinding {
	var doc any
	if err := json.Unmarshal(jsonc.ToJSON(content), &doc); err != nil {
		f := repoFinding(rel, IDJSON, "this JSON does not parse", err.Error())
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			f.Line = lineAt(content, syntax.Offset)
		}
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
	if !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
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
	namespacedLocation  = regexp.MustCompile(`schemaLocation\s*=\s*["']\s*\S+\s+(\S+)[^"']*["']`)
)

// checkXML holds an XML file to the strict org validator and to the schema it
// names. A document that names no schema is a finding, because only its syntax
// gets a check.
func checkXML(path, rel string, content []byte) *TreeFinding {
	if err := xmlvalidator.Validate(bytes.NewReader(content)); err != nil {
		return xmlFinding(rel, "this XML is not well-formed XML 1.1", err)
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
	var schema []byte
	var err error
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		schema, err = fetch(location)
	} else {
		location = filepath.Join(filepath.Dir(path), filepath.FromSlash(location))
		schema, err = os.ReadFile(location)
	}
	if err != nil {
		f := repoFinding(rel, IDXML, "the schema this XML names does not load", err.Error())
		return &f
	}
	resolver := xmlvalidator.SchemaResolver(nil)
	if !strings.Contains(location, "://") {
		resolver = xmlvalidator.FileSchemaResolver(filepath.Dir(location))
	}
	if err := xmlvalidator.ValidateWithSchemaResolver(content, schema, resolver); err != nil {
		return xmlFinding(rel, "this XML breaks the schema it names", err)
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

func fetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// lineAt is the line that holds a byte offset.
func lineAt(content []byte, offset int64) int {
	if offset > int64(len(content)) {
		offset = int64(len(content))
	}
	return bytes.Count(content[:offset], []byte("\n")) + 1
}
