# blamelanguage

A closing message that deflects blame is MARKED. The rule reports the phrases it found, the launcher appends one line to what the reader sees, and nothing goes back to the model.

Carried over from the `no-blame-language` plugin, whose own notes this replaces.

## It marks, and never refuses

This was a Stop hook once. That was wrong the same way `linkrefs` was wrong. A Stop hook runs after the message has streamed, so refusing cannot unsend anything. The reader sees the deflection, then a near-identical retype that explains itself and names the banned phrase again. The guard fires on that too, and asks for a third message with the same property. There is no bound on the loop. One goal was refused nine times over on it.

The reader is the surface this rule is about. So the annotation goes there and the model is left alone. That is the standing rule for a guard here: mitigate rather than refuse.

## Where it runs

`plugins/slopfix/hooks/blame-language.sh` in cc-marketplace execs `slopfix message --only blamelanguage.ID` on **MessageDisplay**. The launcher owns the event plumbing: it accumulates the deltas across flushes, judges the message whole, and renders the annotation. This package owns the detection.

`CC_NO_BLAME_LANGUAGE=0` disables it. Every failure path prints nothing, which leaves the original text on screen. That covers an unparseable payload, an empty one, a message with no banned phrase, and any other event. This runs in the render path. A guard that can eat output is worse than no guard.

## The message is judged whole

A banned phrase can span a line wrap. One flush carries only the lines that completed since the last one. Judging a flush alone therefore misses every phrase that straddles a boundary, and marks the same message repeatedly. The launcher accumulates into a per-message state file keyed by `message_id`, the way `linkrefs` carries its fence state, and drops it on the final flush. Losing that file costs the phrases in the earlier flushes, never a wrong annotation.

The non-streaming path calls the hook once, with `index` 0, `final` true, and the whole message as the delta. It is the same case with no accumulation in front of it.

## `displayContent` is display-only

Read out of the shipped bundle rather than assumed. The event's schema calls it "Text displayed in place of the delta", and states that the hook "replaces the delta on screen without changing the stored message". It is the only field the output schema carries.

## The phrase table is data

`bannedPhrases` is a plain slice, so extending it is editing one list.

- `that predates this session`, `this was existing code`, `i only copied it` and `git blame shows` are the provenance openers named verbatim in `you-wrote-it-own-it.md`.
- `pre-existing` and `preexisting` are the operator's own addition, called out by name as a gap that had to be closed.
- `not related to my change`, `unrelated to my diff` and the `not my *` family are direct synonyms of an entry already listed, kept narrow rather than speculative.

Matching is case-insensitive over whitespace-normalized text. Every run of ASCII whitespace collapses to a single space before a phrase is searched for. A phrase that a wrap split across two lines therefore still matches. Normalization records the source offset of every byte it keeps, so a match in the collapsed string traces back to its real line.

The exemption logic is ported from `linkrefs`, not reinvented. Fenced code, indented code and blockquote lines are exempt. This policy can therefore be documented and discussed without tripping the rule. Inline backticks are NOT exempt.

## What is not banned

Naming a real blocker plainly is never marked: "this needs your call on A vs B, so I pushed the branch with A and left the test red" carries none of the phrases. The rule is about deflection, not about admitting a limit.

## Files

- `blamelanguage.go` -- the phrase table, the compiled case-insensitive matchers, whitespace normalization with offset tracking, and the fenced, indented and blockquote exemption
- `phrases_test.go` -- every banned phrase is found, plus case-insensitivity, normalization across a wrap, and each exemption
- `phrase_test.go`, `blamelanguage_test.go` -- inline backticks are not exempt, and a fixed-and-owned finding and an honest deferral are both clean
