package bashclean

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validateGrep(root, body string) error {
	return ValidateGrepJSON([]byte(`<grepJSON` + root + `>` + body + `</grepJSON>`))
}

func TestTheShippedGrepTableIsValid(t *testing.T) {
	require.NoError(t, ValidateGrepJSON(grepJSONXML))
}

func TestGrepTableFailsOnInvalidContent(t *testing.T) {
	const sensitive = ` case="sensitive"`
	for name, c := range map[string]struct{ root, body, want string }{
		"a sound table":                                  {sensitive, `<flags name="g"><flag short="i" effect="ignore-case"><test cmd="grep -i a x">out</test></flag></flags>`, ""},
		"a root that does not match as typed":            {"", `<test cmd="grep a x" keep="true"/>`, `says case="sensitive"`},
		"two tests of one command":                       {sensitive, `<test cmd="grep a x" keep="true"/><test cmd="grep a x" keep="true"/>`, "repeats the one"},
		"commands that differ only in case are distinct": {sensitive, `<test cmd="grep A x" keep="true"/><test cmd="grep a x" keep="true"/>`, ""},
		"one command printing two outputs":               {sensitive, `<test cmd="grep a x">one</test><program name="grep" flags="g" syntax="basic"><test cmd="grep a x">two</test></program>`, "expects a different result"},
		"one command kept and rewritten":                 {sensitive, `<test cmd="grep a x" keep="true"/><program name="grep" flags="g" syntax="basic"><test cmd="grep a x"/></program>`, "expects a different result"},
		"two programs of one name":                       {sensitive, `<program name="grep" flags="g" syntax="basic"/><program name="grep" flags="g" syntax="basic"/>`, "repeats the one"},
		"two flag sets of one name":                      {sensitive, `<flags name="g"><flag short="i" effect="none"/></flags><flags name="g"><flag short="j" effect="none"/></flags>`, "repeats the one"},
		"two flags of one letter":                        {sensitive, `<flags name="g"><flag short="i" effect="none"/><flag short="i" long="--x" effect="none"/></flags>`, "repeats the one"},
		"two flags differing in case are distinct":       {sensitive, `<flags name="g"><flag short="i" effect="none"/><flag short="I" effect="none"/></flags>`, ""},
		"a flag with neither spelling":                   {sensitive, `<flags name="g"><flag effect="none"/></flags>`, "needs one of short, long"},
		"an effect outside the set":                      {sensitive, `<flags name="g"><flag short="i" effect="shout"/></flags>`, "is not one of"},
		"a syntax outside the set":                       {sensitive, `<program name="grep" flags="g" syntax="regex"/>`, "is not one of"},
		"two fixtures of one name":                       {sensitive, `<fixture name="a.json" kind="none">x</fixture><fixture name="a.json" kind="none">y</fixture>`, "repeats the one"},
		"two translations of one pattern":                {sensitive, `<translate syntax="basic" from="a" to="b"/><translate syntax="basic" from="a"/>`, "repeats the one"},
		"an unknown element":                             {sensitive, `<bogus/>`, "is not an element"},
	} {
		t.Run(name, func(t *testing.T) {
			err := validateGrep(c.root, c.body)
			if c.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}
