# ste: Simplified Technical English

ASD-STE100 is a controlled language. Each approved word has a single meaning and part of speech. Its rules keep each sentence to a single reading. STE governs prose. It applies to a comment or a commit message as much as a document. The text that reaches `Check` is already a block joined to a single line.

The members share the sentence splitter, the masks and the repair pass. They therefore share a package. Each member still selects on its own by ID.

- `ste/contraction`: a contraction. The repair writes the expansion and keeps the capitalization.
- `ste/modal`: `should`, `shall`, `could`, `might` and `would`. The repair writes `must` for obligation and `can` for possibility.
- `ste/semicolon`: the semicolon. Every semicolon has a repair. A period replaces one where the words after it are a sentence. The next word takes a capital. After a colon, each later item of a list opens a sentence behind `This also covers`. The item's own `and` or `or` goes. Items that each pair a case with an answer, as in `for a parser, round-trip`, become a list of pairs with each answer in a parenthesis. Any other semicolon becomes a comma, because the words after it describe the words before it.
- `ste/comma-splice`: a comma that joins clauses that each stand alone. The repair writes a period. `However,` replaces `but` and `yet`. The repair drops `and` and `so`. The words before the comma must hold a main clause, and the subject after it must agree with its verb.
- `ste/sentence-length`: a sentence over `25` words, the STE cap for a description. The repair divides it at a clause boundary that the `syntax` parser finds. With no such boundary, it divides between words near the cap. Each half must be a grammatical sentence. After a colon, the words since the colon must hold a clause too. A restated subject must belong to the clause right before the cut, with no other finite verb between them. Where no clause boundary gives sentences, a carrier writes the rest as a sentence. The list below names each carrier. The repair divides again until every sentence is under the cap.
- `ste/postdeterminer`: a numeral between a determiner and its noun, as in `the three rules`. The repair cuts the numeral. A unit, a percent, a year, a status code, `any` and `first` keep theirs. So does a numeral that heads its phrase, as in `the two cannot` and `this one`. Fix leaves the words inside a quotation alone, as Check reads them.
- `ste/count`: a stated count anywhere in the line, read with the `Gate` substrate. The repair takes the number out after the join, with the `counts` rewording.

The warning rules read patterns that need a person to repair. A warning never fails `check`. `check --json` gives each finding a `severity` of `error` or `warning`. The language server sends a warning at warning level.

- `ste/instruction-length`: a sentence of `21` to `25` words, over the STE cap for an instruction.
- `ste/passive`: a form of `be` and a past participle, as the parser tags them.
- `ste/noun-cluster`: a run of nouns longer than `NounClusterCap`, which is `3`.
- `ste/tense`: a perfect or a progressive tense. The approved `-ing` words, such as `missing` and `during`, do not count.
- `ste/dictionary`: a word the STE dictionary does not approve, with its approved replacements. The table is the `dictionary` list in `rules/ste-dictionary.xml`. A `<list>` holds a string on each line of its text and needs no `<test>`. It holds only words with no approved sense, because a match by spelling cannot tell senses apart.
- `ste/paragraph-length`: a paragraph with more sentences than `ParagraphSentenceCap`, which is `6`. A list item never counts.

How a long sentence divides:

- A clause after `and` or `but` that names its own subject starts the new sentence. `but` and `yet` become `However,`.
- A clause after `, so` divides when it names its own subject, and `so` drops. A `so` with no comma states a purpose, and divides only behind a carrier.
- A verb group that shares the subject gets it again. A long subject becomes an agreeing pronoun.
- A closing `, which` clause opens with `This`. A closing `because` clause opens with `This is because`.
- A list, a quotation and a subordinate clause never divide at a clause boundary.
- The first half must hold a main clause. A sentence that opens on `If`, `When` or a `to` infinitive keeps that clause with its main clause after the comma.
- No division leaves part of a dash aside on each side. A division at the closing dash ends the aside with the sentence.
- A sentence with no boundary divides between words, as close to the cap as a good place allows. A comma. Then a word that opens a phrase, rank first. The first part never ends on a word that opens what it leaves out.
- The forced division never lands inside a code span, a link, a quotation or a parenthesis. A bold run that opens on the `and` the division drops opens the rest instead. The rest opens as it stands when it names its own subject or opens on an imperative. So does a clause after a dash or a colon, and an instruction after `, so`. A finite verb gets the subject again.
- Any other rest goes behind a carrier (`ste/carrier.go`). A trailing adverbial takes `This happens`, `This holds`, `Do this` or `This applies`, by the main verb. A phrase of place, a participle or a relative clause after a noun restates the noun: `That barn is behind the hills`. The rest of a list takes `It also covers`, or the subject and `also` before a list of verb groups. A purpose after an instruction takes `Do this so`.
- An opening subordinate clause or infinitive moves behind its main clause: `Y. This happens if X.` A main clause can point back into it, as `those files` does. That main clause keeps the order: `Suppose X. Then Y.` A long subject goes into a sentence of its own: `Consider a reader arriving at X. That reader still deserves Y.`

What `ste` does not flag:

- An inline code span, a link target and an HTML entity are masked. Every repair leaves them as they are.
- A comma splice needs a subject and a finite verb after the comma. A list, an Oxford comma, a participle and an infinitive do not qualify. Without a conjunction, the words before the comma must be a main clause.
- No repair rewrites inside a quotation.
- Text in parentheses counts as a single word. A citation thus cannot inflate a sentence.
- A period ends a sentence only when what follows opens the next. That rules out `e.g.` and `$(...)`. A file name or a section mark can open a sentence in lower case.
- `ste/count` exempts arithmetic, such as a range or an expression. It exempts no noun, because a duration or a size goes stale too.
- The literal shapes `issue #<digits>`, `Vega <digits>`, `8 bits`, `16 bits`, `32 bits` and `64 bits` are not counts. A number that picks an item from a list, as in `gate 5`, is a count.
- A cut keeps the sentence English. `over` and `above` always stay, so `over 32 banks` becomes `over many banks`. A capital moves only at the start of a sentence.
