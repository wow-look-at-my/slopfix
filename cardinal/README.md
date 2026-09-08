# cardinal

Decides whether a number in a piece of text is a stated count: a number that is true today and wrong after the next commit.

This package holds no rule ID of its own. It is the shared half of two rules that report against different substrates, `counts/inventory-count` over a document and `comments/number` over a source comment. Each of those packages keeps its substrate: which text carries the author's own voice, and where a finding lands.

## The substrate is the policy

A `Substrate` says how much a piece of text has to claim before a number in it counts as a tally.

| | `Prose` | `Comment` |
| --- | --- | --- |
| Frame required | yes | no |
| Vocabulary | cardinals from two upward | cardinals, ordinals, scales, repeat counts |
| Exemptions | none | status code, section sign, currency sign |

Prose requires a frame because a document legitimately carries numbers that count nothing: a version, a port, an example. The frame is a sentence claiming the things belong here, spelled as a possessive, a having verb, or a pointer into the page.

A comment requires no frame, because a number written beside code is nearly always a count of what the code holds. The exemptions carry what such a number can be instead. The token's own shape carries the rest. A URL is never a count. A qualified name names itself.

## Reading it

```go
for _, tok := range cardinal.Find(line, cardinal.Comment) {
	report(tok.Offset, tok.Text)
}
```

`Find` returns what the substrate's repair needs. Prose returns the whole quantity, so `cardinal.Leading` can cut the number off the front of it. A comment returns the number alone, because there is nothing there to cut.
