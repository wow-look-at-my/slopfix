// ste/sentence-length: a sentence over the STE word cap.

package slopfix

import _ "embed"

import "github.com/wow-look-at-my/slopfix/ste"

// ste/sentence-length: a sentence over the STE word cap. The repair divides it
// at a clause boundary, and between words only where no boundary reads.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDSentenceCap,
		Category: RuleSTE,
		Detect:   detectSentenceCap,
		Autofix:  autofixSentenceCap,
		Cases: []RuleCase{
			{Name: ste.IDSentenceCap, Path: "x.md", Text: "This sentence carries far more words than any reader can hold in mind at one time and it keeps going well past the cap of the rule today.\n"},
			{
				Name: "a relative clause stays with its noun", Path: "x.md",
				Text: "The reader answers the first prose paragraph of text as one restated sentence that carries a later \"that\" clause with the verb that governs it, laid out under a leading heading where both fit.\n",
				Want: "The reader answers the first prose paragraph of text as one restated sentence that carries a later \"that\" clause with the verb. That verb governs it, laid out under a leading heading where both fit.\n",
			},
			{
				Name: "a colon before a clause with its own subject", Path: "x.md",
				Text: "A sentence wraps across the lines of one replacement, so they come back together: a line fix wrote beside a line the edit wrote reads as neither.\n",
				Want: "A sentence wraps across the lines of one replacement, so they come back together. A line fix wrote beside a line the edit wrote reads as neither.\n",
			},
			{Name: "a carrier never opens on a conjunction", Path: "governor.cpp", Text: fixtureHeader + carrierConjunctionCase, Want: fixtureHeader + carrierConjunctionWant},
			{Name: "a capital never renames a variable", Path: "governor.cpp", Text: fixtureHeader + variableOpensCase, Want: fixtureHeader + variableOpensWant},
			{Name: "a rule around comment prose stays at the edges", Path: "governor.cpp", Text: fixtureHeader + ruledCommentCase, Want: fixtureHeader + ruledCommentWant},
			{Name: "the objects of a preposition stay joined", Path: "CMakeLists.txt", Text: coordinatedObjectsCase, Want: coordinatedObjectsWant},
			{Name: "a clause of manner stays with its sentence", Path: "ident.go", Text: fixtureHeader + mannerClauseCase, Want: fixtureHeader + mannerClauseWant},
			{Name: "a list with no verb divides behind a carrier", Path: "moments.go", Text: fixtureHeader + verblessListCase, Want: fixtureHeader + verblessListWant},
			{Name: "a plural noun after a colon opens a subject of its own", Path: "schema.go", Text: fixtureHeader + colonPluralSubjectCase, Want: fixtureHeader + colonPluralSubjectWant},
			{Name: "a phrase cut from its verb stands behind a carrier", Path: "pinned.go", Text: fixtureHeader + strandedPhraseCase, Want: fixtureHeader + strandedPhraseWant},
		},
	})
}

// mannerClauseCase holds a sentence over the cap that ends on a clause of
// manner. "As a variable such as q is" stands as no sentence, so the division
// lands at ", or", where the clause after it stands.
const mannerClauseCase = `package ident

import "strings"

// lowerIdentifier reports a word that reads as a name in code. It carries a capital, a digit, an underscore or a dot inside it, or it is a single letter other than the article, as a variable such as q is.
func lowerIdentifier(word string) bool {
	if word == "" {
		return false
	}
	if len(word) == 1 {
		return word != "a"
	}
	for _, r := range word[1:] {
		if r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return strings.ToLower(word) != word
}
`

const mannerClauseWant = `package ident

import "strings"

// lowerIdentifier reports a word that reads as a name in code. It carries a capital, a digit, an underscore or a dot inside it. Otherwise, it is a single letter other than the article, as a variable such
// as q is.
func lowerIdentifier(word string) bool {
	if word == "" {
		return false
	}
	if len(word) == 1 {
		return word != "a"
	}
	for _, r := range word[1:] {
		if r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return strings.ToLower(word) != word
}
`

// colonPluralSubjectCase holds a sentence over the cap whose clause after the
// colon opens on a plural noun the tagger reads as a verb. "models such as
// Prisma cache" names its own subject, so the division writes no subject in
// front of it. The words after the colon are under the cap, so they stay one
// sentence.
const colonPluralSubjectCase = `package schema

// The schema is checked after a migration, not at a new connection: models such as Prisma cache earlier schemas at every connection by design, so a connection-only probe scores every server as stale.
const (
	cacheDir     = ".cache/schema/outputs"
	lockFile     = ".cache/schema/outputs.lock"
	manifestFile = ".cache/schema/manifest.json"
	staleMarker  = ".cache/schema/stale"
	readTimeout  = "30s"
	writeTimeout = "60s"
	maxAgeSecs   = 3600
	maxEntries   = 4096
)
`

const colonPluralSubjectWant = `package schema

// The schema is checked after a migration, not at a new connection. Models such as Prisma cache earlier schemas at every connection by design, so a connection-only probe scores every server as stale.
const (
	cacheDir     = ".cache/schema/outputs"
	lockFile     = ".cache/schema/outputs.lock"
	manifestFile = ".cache/schema/manifest.json"
	staleMarker  = ".cache/schema/stale"
	readTimeout  = "30s"
	writeTimeout = "60s"
	maxAgeSecs   = 3600
	maxEntries   = 4096
)
`

// strandedPhraseCase holds a sentence over the cap whose only cut under the
// cap opens on "for" with no mark before it. The phrase belongs to the verb in
// front of the cut, so the rest stands behind "This is".
const strandedPhraseCase = `package pinned

import "strings"

// pinnedName reports a name that the file or the repository holds in lower case alone, as a config file holds "bazel" for the tool prose calls Bazel.
func pinnedName(text, name string) bool {
	lower := strings.ToLower(name)
	if lower == name {
		return false
	}
	for _, word := range strings.Fields(text) {
		if strings.Trim(word, "\"'.,;:") == lower {
			return true
		}
	}
	return false
}
`

const strandedPhraseWant = `package pinned

import "strings"

// pinnedName reports a name that the file or the repository holds in lower case alone, as a config file holds "bazel". This is for the tool prose
// calls Bazel.
func pinnedName(text, name string) bool {
	lower := strings.ToLower(name)
	if lower == name {
		return false
	}
	for _, word := range strings.Fields(text) {
		if strings.Trim(word, "\"'.,;:") == lower {
			return true
		}
	}
	return false
}
`

// verblessListCase holds a list over the cap that names terms and holds no
// verb, as the terms of a formula do. It closes at the last comma under the
// cap, and the rest stands behind "This also covers".
const verblessListCase = `package moments

// Per octant, total mass, mean position, second moment, weighted color, merged opacity, luma variance per cube, and flatness ratio of the stack against an opaque layer of the same cube.
func octantMoments(masses, positions, colors []float64) (mass, mean, color float64) {
	for i, m := range masses {
		mass += m
		mean += m * positions[i]
		color += m * colors[i]
	}
	if mass > 0 {
		mean /= mass
		color /= mass
	}
	return mass, mean, color
}
`

const verblessListWant = `package moments

// Per octant, total mass, mean position, second moment, weighted color, merged opacity, luma variance per cube. This also covers flatness ratio of the stack against an opaque layer of
// the same cube.
func octantMoments(masses, positions, colors []float64) (mass, mean, color float64) {
	for i, m := range masses {
		mass += m
		mean += m * positions[i]
		color += m * colors[i]
	}
	if mass > 0 {
		mean /= mass
		color /= mass
	}
	return mass, mean, color
}
`

// carrierConjunctionCase holds a noun phrase over the cap whose list after a
// colon ends on ", and". A carrier in front of "and the scalars it set" reads
// as no sentence, so the division lands at the colon.
const carrierConjunctionCase = `struct Governor {
  double lodBias_ = 1.0;
  // Pair-demand elasticity per rung, and the act whose demand response
  // measures one of them: the demand and scalars before the act, and the
  // scalars it set.
  struct {
    double eLod = kGovDemandElasticityPrior;
    double eRes = kGovDemandElasticityPrior;
    bool armed = false;
    double demand = 0, lod = 1, res = 1;
    double actLod = 1, actRes = 1;
  } probe_;
};
`

const carrierConjunctionWant = `struct Governor {
  double lodBias_ = 1.0;
  // Pair-demand elasticity per rung, and the act whose demand response
  // measures one of them. This is the demand and scalars before the act,
  // and the scalars it set.
  struct {
    double eLod = kGovDemandElasticityPrior;
    double eRes = kGovDemandElasticityPrior;
    bool armed = false;
    double demand = 0, lod = 1, res = 1;
    double actLod = 1, actRes = 1;
  } probe_;
};
`

//go:embed testdata/variable-opens-case.cpp
var variableOpensCase string

//go:embed testdata/variable-opens-want.cpp
var variableOpensWant string

//go:embed testdata/ruled-comment-case.cpp
var ruledCommentCase string

//go:embed testdata/ruled-comment-want.cpp
var ruledCommentWant string

// coordinatedObjectsCase holds a CMake comment whose list after a colon joins
// clauses with semicolons. "on the copy and the in-place row wires" names a
// couple of objects of "on", so no division lands at its "and". Each item that
// is a clause of its own opens a sentence as it stands.
const coordinatedObjectsCase = `# animation-prepack-equivalence - the animation pre-pack's byte gate: every
# adopted frame of a synthetic clip lands records, cubeBuf, C and a rendered
# image equal to the live async tick wire's, on the copy and the in-place
# (UMA direct) row wires; a repeated C reuses its pipeline triple; an edge
# repack of an adopted frame and a resumed live tick match too. The parking
# leg packs a real multi-round chain at the default chunk clamp.
add_test(NAME animation-prepack-equivalence
  COMMAND $<TARGET_FILE:splat-vulkan> ${PARKING} --max-splats 5000 --fgen-test prepack
          --cube-edge 1 --validate)
`

const coordinatedObjectsWant = `# animation-prepack-equivalence - the animation pre-pack's byte gate. Every
# adopted frame of a synthetic clip lands records and cubeBuf. It also lands
# C and a rendered image equal to the live async tick wire's, on the copy
# and the in-place (UMA direct) row wires. A repeated C reuses its pipeline
# triple. An edge repack of an adopted frame and a resumed live tick match
# too. The parking leg packs a real multi-round chain at the default chunk
# clamp.
add_test(NAME animation-prepack-equivalence
  COMMAND $<TARGET_FILE:splat-vulkan> ${PARKING} --max-splats 5000 --fgen-test prepack
          --cube-edge 1 --validate)
`

// detectSentenceCap answers each sentence over the word cap that the case
// carries.
func detectSentenceCap(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDSentenceCap)
}

// autofixSentenceCap divides the over-long sentence at a clause boundary, and
// answers the case without the finding.
func autofixSentenceCap(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDSentenceCap)
}
