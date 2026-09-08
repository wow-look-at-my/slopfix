# cardinal

Decides whether a number in a piece of text is a stated count: a number that is true today and wrong after the next commit.

This package holds no rule ID of its own. It is the shared half of three rules. Each reports against its own substrate: `counts/inventory-count` over a document, `ste/count` over the same document for the merge gate, and `comments/number` over a source comment. Each of those packages keeps its substrate: which text carries the author's own voice, and where a finding lands.

## The substrate is the policy

A `Substrate` says how much a piece of text has to claim before a number in it counts as a tally.

| | `Prose` | `Gate` | `Comment` |
| --- | --- | --- | --- |
| Rule | `counts/inventory-count` | `ste/count` | `comments/number` |
| Shape | quantity | quantity | number |
| Frame required | yes | no | no |
| Vocabulary | two upward, plus a dozen | two to twelve | cardinals, ordinals, scales, repeat counts |
| Exemptions | function-word gap, a longer number | unit, arithmetic | status code, section sign, currency sign |

Prose requires a frame because a document legitimately carries numbers that count nothing: a version, a port, an example. The frame is a sentence claiming the things belong here, spelled as a possessive, a having verb, or a pointer into the page.

The gate reads the same document and asks for no frame. It pays for that with a list of units, so a size and a duration are measured rather than counted. That is why `The read has 20 seconds.` is a finding for `Prose` and exempt for `Gate`. The two disagree on purpose. Folding the disagreement away moves verdicts on every repository the gate reads.

A comment requires no frame either, because a number written beside code is nearly always a count of what the code holds. The exemptions carry what such a number can be instead. The token's own shape carries the rest. A URL is never a count. A qualified name names itself.

## The vocabularies are not derived from each other

Each list is what its rule has always matched. Widening one to match another changes the verdict on text nobody has edited, which is why they sit here as three lists rather than one.

## Reading it

```go
for _, tok := range cardinal.Find(line, cardinal.Comment) {
	report(tok.Offset, tok.Text)
}
```

`Find` returns what the substrate's repair needs. Prose returns the whole quantity, so `cardinal.Leading` can cut the number off the front of it. A comment returns the number alone, because there is nothing there to cut.
