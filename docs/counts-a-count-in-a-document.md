# counts: a count in a document

`counts/inventory-count` cuts the cardinal out of a sentence that counts what is here. `there are three sections` becomes `there are sections`, which stays true. The org rules that a count in markdown is not worth maintaining.

Every reported count gets a repair. Where a bare cut breaks the sentence, `counts/reword.go` writes words that state no figure. A rate becomes `every few`. A cap becomes `a bounded number of`, whether words (`at most`, `max`) or a sign (`<=`, `<`) state it. A sign that sets a floor (`>=`, `>`) reads as a unit does. A unit takes `a couple of`, `a few`, `several` or `many`, by size.

After a preposition or a noun the number becomes `multiple`.

A hedge such as `about` or `exactly` goes with the number. A zero becomes `no`. A zero that is the subject of a verb names a value, and becomes `a value of zero`: `0 disables chunking` reads `a value of zero disables chunking`. A label goes stale like a count. It is a finding too, and `counts/label.go` names the item instead. Digits after a singular noun name an item, as in `branch 3 sees` (`cardinal.NamesAnItem`).

The repair takes the words the text gives that item, as in `branch (3) the keyless fallback` or `Keyless fallback (branch 3)`. With none, it writes `one id` or `a later branch`. A plural noun after the label is what the label describes: `rule 6 inputs` becomes `the inputs of a later rule`. A point sits after `at`, `by` or `to`, before a noun that is not a unit (`cardinal.APoint`). In a comment it takes the name of a constant with that value, in backticks. Otherwise it takes `a set number of` before a noun, or `a set limit`, `a set value` or `a set amount`.

A bare number that the same clause sets beside it, as in `warns at 500 lines and errors at 750`, says how it compares: `a higher count`, `a lower count` or `the same count`. No clause stays half converted.

A count needs a frame and a quantity on the same line. The quantity is a cardinal that governs a plural noun. The frame is a possessive (`this repo's plugins`), a having verb (`it ships hooks`) or a deictic (`the rules below`). A quantity with no frame is ordinary technical prose.

It does not flag the singular, which is overwhelmingly a pronoun in English. That gap is stated rather than closed. It drops a quantity reached through a function word, as in `2 of the format drops`. It drops a match that continues a longer number, such as a version. Fences, tables, headings and backtick spans never reach it. A measurement inside a frame IS reported, because the number belongs where it is enforced.

A number behind a section sign cites a section, so neither count rule reads it. `counts/section-number` repairs it instead. A section number goes stale when a section is inserted above it. The repair writes a link named by a slug of the heading's title: `§9` becomes `[§owning-renderer](#9-owning-the-renderer)`. The slug keeps the first `slugWords` words that name the section. A repeated slug gets `-2`, and the anchor follows GitHub.

The prose rules read the link as the citation it replaced, which holds no word. A file named right before the citation, or after it with `of`, resolves beside the document. A citation that no numbered heading answers keeps its number.
