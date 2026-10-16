package autoallow

import "github.com/wow-look-at-my/slopfix/xmlcheck"

// Spec is what rules.xml is held to. It names shell words, flags and paths,
// which a shell reads as typed, so the root says case="sensitive" and no entry
// can say otherwise.
func Spec() xmlcheck.Spec {
	bool2 := []string{"true", "false"}
	flagList := xmlcheck.Elem{Children: []string{"flag"}}
	stringList := xmlcheck.Elem{Children: []string{"value"}}
	text := xmlcheck.Elem{Text: true}
	elems := map[string]xmlcheck.Elem{
		"rules":     {Case: true, Children: []string{"test", "allow", "ask", "deny", "denyPath", "mcpServer"}},
		"test":      {Required: []string{"cmd"}, Present: []string{"expect"}},
		"allow":     {Children: []string{"rule"}},
		"ask":       {Children: []string{"rule"}},
		"deny":      {Children: []string{"rule"}},
		"denyPath":  {Required: []string{"prefix"}, Text: true, Keys: []xmlcheck.Key{{Attrs: []string{"prefix"}}}},
		"mcpServer": {Required: []string{"name"}, Children: []string{"tool"}, Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"tool":      {Text: true, Keys: []xmlcheck.Key{{Attrs: []string{"#text"}}}},
		"rule": {
			Any: []string{"name", "command"},
			Attrs: []string{
				"description", "inlineScript", "evalFlags", "evalSubcommands", "message",
				"allowedFlags", "denyWithMessage", "helpAlwaysAllowed", "bareOnly",
			},
			Enum: map[string][]string{"inlineScript": bool2, "helpAlwaysAllowed": bool2, "bareOnly": bool2},
			Children: []string{
				"test", "allowedFlags", "deniedFlags", "execFlags", "requiredFlags", "flagsWithValue",
				"denyArgSubstrings", "refuse", "allowedArgPrefixes", "requireFlagValue", "rule",
			},
			Keys: []xmlcheck.Key{{Attrs: []string{"name"}, Split: true}, {Attrs: []string{"command"}}},
		},
		"allowedFlags":       flagList,
		"deniedFlags":        flagList,
		"execFlags":          flagList,
		"requiredFlags":      flagList,
		"flagsWithValue":     flagList,
		"flag":               {Required: []string{"name"}, Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"denyArgSubstrings":  stringList,
		"allowedArgPrefixes": stringList,
		"value":              {Text: true, Keys: []xmlcheck.Key{{Attrs: []string{"#text"}}}},
		"refuse": {
			Required: []string{"message"},
			Children: []string{"flag", "flagValue", "argPrefix", "argSubstring"},
		},
		"flagValue":        {Children: []string{"flag", "value"}},
		"argPrefix":        text,
		"argSubstring":     text,
		"requireFlagValue": {Attrs: []string{"default"}, Children: []string{"flag", "allowed"}},
		"allowed":          text,
	}
	return xmlcheck.Spec{
		Root:      "rules",
		Elems:     elems,
		Test:      xmlcheck.TestDef{Elem: "test", In: "cmd", Out: []string{"expect"}, EmptyOut: true},
		Sensitive: true,
	}
}

// Validate holds a rules file to Spec.
func Validate(data []byte) error {
	return xmlcheck.Run(Spec(), xmlcheck.File{Name: "autoallow/rules.xml", Raw: data})
}

// init stops the binary at start-up when the embedded table is invalid, rather
// than letting a rule that cannot fire sit in it.
func init() {
	if err := Validate(rulesXML); err != nil {
		panic("autoallow: " + err.Error())
	}
}
