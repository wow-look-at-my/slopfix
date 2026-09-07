# laziness

A turn that ends with the work undone, because doing the work is the harder path. This rule reads the closing message and reports it.

## What it rejects

A closing message that reports a defect the session found and left in place. Or one that asks permission to carry on instead of carrying on.

Those were judged apart before. They are the same thing. The turn ends with the work undone. The reader is left holding it. Folding them puts the shapes in one table and gives the consumer one answer to act on.

The table matches the surface of each. Disowning a defect you found. Handing a repair back to the reader. Leaving a defect in place. Excusing yourself from a repair. Announcing an attribution hunt instead of a repair. Offering authorship in place of a fix. Asking permission in place of acting.

## Why

A session found a build phase that failed on every machine with bubblewrap installed, and an instruction file carrying the same bullet twice. It diagnosed both correctly and verified both with a negative control. It wrote them up under a heading saying they were worth the reader's attention, and closed the turn with the line "neither mine to fix unasked". The owner replied that fixing it is the job.

A second session hit a failing test and spent a full run confirming the failure happened on the unmodified base commit too. It reported the failure as pre-existing and closed the turn there. The owner had to force it to keep going. Two further runs found the real cause in minutes.

The permission half has its own history. The refusal it answers with used to run four paragraphs, arguing autonomy and liability. That handed the model a case to argue back with, and the turn went on the argument instead of the work.

## Worked examples

A punt, and what the consumer does with it.

```
$ printf 'I found a broken build phase. Neither mine to fix unasked.\n' | slopfix message
1: [laziness/punt] disowning a defect you found: "Neither mine to fix unasked."
exit=1
```

The same finding, owned instead.

```
$ printf 'I found a broken build phase. I fixed it and pushed the fix.\n' | slopfix message
exit=0
```

A question whose answer was already yes.

```
$ printf 'Want me to fix it?\n' | slopfix message --json
{"findings":[{"id":"laziness/punt","tell":"asking permission in place of acting","sentence":"Want me to fix it?","line":1}]}
```

## What it does not flag

A pardon excuses the sentence it sits in. A sentence carrying a shape AND saying the repair happened is not a punt. Claiming the fix in a different sentence pardons nothing, because the shape is still the reason that sentence exists.

A bare diagnosis is legitimate. Naming a failure as pre-existing is real work. That word alone is therefore absent from the table. What fires is the diagnosis offered as the reason for stopping.

An honest deferral is not a punt. A message naming a real blocker plainly, and saying what was pushed in its place, carries none of these shapes.

Fenced code, indented code and a blockquote are exempt. This policy can therefore be written down without tripping the rule. An inline backtick span is not exempt, matching the sibling message guards: a punt in backticks is still the writer's own voice.

A sentence is reported for a single shape, whichever found it. A message repeating itself must not read as a worse offence than it is.

## Repair

It reports only. A closing message is not a file to rewrite. The repair is work rather than words. What the consumer does with a finding is resume the turn.

## Rule ID, and running it

The ID is `laziness/punt`.

```sh
slopfix message < message.txt
slopfix message --json < message.txt
```

The command exits 1 when it finds anything, and 0 when it does not. The other commands judge a file. This judges the text the model sends to the reader, which is never on disk.

## Which hook selects it

The `no-laziness` plugin, at Stop.

The event is the exception to the migration the sibling guards took. A wording guard moved to MessageDisplay because its message has already streamed, so refusing cannot unsend it. This rule exists to stop the model STOPPING, and only a Stop hook can do that. An annotation under an abandoned turn changes nothing about the turn being abandoned.

The hook answers a finding with the single word the reader types back. It does not lecture. There is nothing in that answer to argue with.
