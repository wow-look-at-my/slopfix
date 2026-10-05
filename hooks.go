package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/askproperly"
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// IDInventoryCount names the stale-count rule over a document.
const IDInventoryCount = counts.ID

// IDCommentNumber names the stale-count rule over a comment.
const IDCommentNumber = commentfix.ID

// IDPunt names the rule over a closing message.
const IDPunt = laziness.ID

// IDBlame names the deflection rule over a closing message.
const IDBlame = blamelanguage.ID

// IDAsk names the rule over a decision handed to the reader in prose.
const IDAsk = askproperly.ID

// Hook is a named selection of rule IDs. Only holds a category or a rule ID,
// and empty means the default set.
type Hook struct {
	Name    string
	Event   string
	Runs    string
	Summary string
	Only    []string
	Pending bool
}

// hooks is the mapping, and the only copy of it. A test asserts that every rule
// appears here and that every entry names a rule that exists.
var hooks = []Hook{
	{
		Name:    "no-counts-in-docs",
		Event:   "PreToolUse",
		Runs:    "slopfix hook --only counts",
		Summary: "a cardinal stated about what is here, in a document",
		Only:    []string{string(RuleCounts)},
	},
	{
		Name:    "no-tombstones",
		Event:   "PreToolUse",
		Runs:    "slopfix hook --only tombstones --max-comment-lines <n>",
		Summary: "a comment describing a state the code has left",
		Only:    []string{string(RuleTombstones)},
	},
	{
		Name:    "common-checks",
		Event:   "PreToolUse",
		Runs:    "slopfix check --json --path <file>",
		Summary: "the default check set, which is what this gate means",
	},
	{
		Name:    "go-toolchain comments phase",
		Event:   "build",
		Runs:    "slopfix check .",
		Summary: "what a comment gets wrong, in any language the adapter knows",
		Only:    []string{IDCommentNumber, commentfix.IDTail, commentfix.IDLength, ste.IDSentenceCap},
	},
	{
		Name:    "ci check",
		Event:   "build",
		Runs:    "slopfix check .",
		Summary: "what a repository keeps, judged from its root, and each uses: ref on a feature branch",
		Only:    []string{string(RuleRepo), workflow.IDBranchPin},
	},
	{
		Name:    "no-laziness",
		Event:   "Stop",
		Runs:    "slopfix hook --only laziness",
		Summary: "a turn ending with the work undone",
		Only:    []string{IDPunt},
	},
	{
		Name:    "no-blame-language",
		Event:   "MessageDisplay",
		Runs:    "slopfix hook --only blame-language",
		Summary: "deflecting phrasing in a closing message",
		Only:    []string{IDBlame},
	},
	{
		Name:    "ask-properly",
		Event:   "MessageDisplay",
		Runs:    "slopfix hook --only ask-properly",
		Summary: "a decision handed to the reader in prose",
		Only:    []string{IDAsk},
	},
	{
		Name:    "link-all-refs",
		Event:   "MessageDisplay",
		Summary: "a pull request, a commit or a branch named without a link",
		Pending: true,
	},
}

// Hooks returns the mapping, in the order a reader reads it.
func Hooks() []Hook { return hooks }

// FindHook answers with the hook of that name.
func FindHook(name string) (Hook, bool) {
	for _, hook := range hooks {
		if hook.Name == name {
			return hook, true
		}
	}
	return Hook{}, false
}

// Selects resolves a hook to the rule IDs it runs.
//
// A pending hook selects nothing, because its rule lives elsewhere. An entry
// naming a category expands to every rule inside it. Naming none at all is the
// default set.
func Selects(hook Hook) set.Set[string] {
	if hook.Pending {
		return set.New[string]()
	}
	if len(hook.Only) == 0 {
		return AllIDs()
	}
	ids := set.New[string]()
	for _, name := range hook.Only {
		if inside := IDsFor(Rule(name)); !inside.IsEmpty() {
			ids = ids.Union(inside)
			continue
		}
		ids.Add(name)
	}
	return ids
}

// EveryRuleID is every rule this repository reports, whichever command reaches
// it. The check path answers for part of it, and the repair categories and the
// comment rule answer for the rest.
func EveryRuleID() set.Set[string] {
	every := AllIDs()
	for _, rule := range AllRules {
		every = every.Union(IDsFor(rule))
	}
	every.AddRange(IDCommentNumber, IDPunt, IDBlame, IDAsk)
	return every
}
