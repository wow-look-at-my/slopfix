package autoallow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validate(root, body string) error {
	return Validate([]byte(`<rules` + root + `>` + body + `</rules>`))
}

func TestTheShippedRulesAreValid(t *testing.T) {
	require.NoError(t, Validate(rulesXML))
}

func TestRulesFailOnInvalidContent(t *testing.T) {
	const sensitive = ` case="sensitive"`
	for name, c := range map[string]struct{ root, body, want string }{
		"a sound table":                                  {sensitive, `<allow><rule name="ls" allowedFlags="*"/></allow><test cmd="ls" expect="allow"/>`, ""},
		"a root that does not match as typed":            {"", `<allow><rule name="ls"/></allow>`, `required attribute "case" is missing`},
		"an entry that ignores case":                     {sensitive, `<allow><rule name="ls"/></allow><test cmd="ls" expect="allow" case="insensitive"/>`, `unexpected attribute "case"`},
		"two tests of one command":                       {sensitive, `<test cmd="ls" expect="allow"/><test cmd="ls" expect="allow"/>`, "repeats the one"},
		"commands that differ only in case are distinct": {sensitive, `<test cmd="LS" expect=""/><test cmd="ls" expect="allow"/>`, ""},
		"one command expecting two answers":              {sensitive, `<allow><rule name="ls"><test cmd="ls" expect="allow"/></rule></allow><test cmd="ls" expect=""/>`, "expects a different result"},
		"a test with no expectation":                     {sensitive, `<test cmd="ls"/>`, `required attribute "expect" is missing`},
		"two rules of one name":                          {sensitive, `<allow><rule name="ls"/><rule name="ls"/></allow>`, "repeats the one"},
		"a name listed in another rule":                  {sensitive, `<allow><rule name="ls"/><rule name="dir, ls"/></allow>`, "repeats the one"},
		"one name in two sections":                       {sensitive, `<allow><rule name="ls"/></allow><deny><rule name="ls"/></deny>`, ""},
		"a rule with neither name nor command":           {sensitive, `<allow><rule description="x"/></allow>`, "needs one of name, command"},
		"two flags of one name":                          {sensitive, `<allow><rule name="ls"><allowedFlags><flag name="-a"/><flag name="-a"/></allowedFlags></rule></allow>`, "repeats the one"},
		"two servers of one name":                        {sensitive, `<mcpServer name="g"><tool>a</tool></mcpServer><mcpServer name="g"/>`, "repeats the one"},
		"two tools of one name":                          {sensitive, `<mcpServer name="g"><tool>a</tool><tool>a</tool></mcpServer>`, "repeats the one"},
		"two paths of one prefix":                        {sensitive, `<denyPath prefix="/x">no</denyPath><denyPath prefix="/x">no</denyPath>`, "repeats the one"},
		"an unknown element":                             {sensitive, `<bogus/>`, `"bogus"`},
		"an unknown attribute":                           {sensitive, `<allow><rule name="ls" color="red"/></allow>`, `unexpected attribute "color"`},
		"a boolean outside its set":                      {sensitive, `<allow><rule name="ls" bareOnly="maybe"/></allow>`, `attribute "bareOnly"`},
	} {
		t.Run(name, func(t *testing.T) {
			err := validate(c.root, c.body)
			if c.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}
