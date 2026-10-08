# repo: what a repository keeps

These rules judge the tree. Only a walk whose root holds `.git` reaches them. `check` reports them. `fix` applies them.

- `repo/agents-file`: a root `CLAUDE.md` that holds more than the `@AGENTS.md` import. `fix` moves its body into `AGENTS.md` and leaves `CLAUDE.md` as `@AGENTS.md` and a newline. Claude Code reads `CLAUDE.md`. Every other agent reads `AGENTS.md`.
- `repo/budget`: a root `README.md` or `AGENTS.md`, a `CLAUDE.md` anywhere, or a `.md` in a `claude_snippets/` directory, over `40000` characters. The repair writes `docs/` beside the file. The count is characters, because a byte count inflates a file with an em dash. `fix` moves the largest `##` sections into `docs/<heading>.md` until the file is at `32000` or less. The gap leaves room for the next edit. The text moves word for word, and each heading under it rises one level. The heading stays, with a link to the new file. A name that exists gets a `-2` suffix. With no `##` section left to move, the sections of the other heading levels move. A file with no heading at all moves its tail into `docs/<name>-continued.md` and keeps a link. A cut inside a fence closes the fence. The moved part opens it again.
- `repo/package-scripts`: a `package.json` with a `scripts` key. A `justfile` holds the commands instead. `fix` writes each script as a recipe in a `justfile` beside the manifest, and deletes the key. The recipe runs the command as written, with `node_modules/.bin` first on `PATH`. A `pre` or `post` script runs around its own, and `npm run x` becomes `just x`. A `package.json` that does not parse is also a finding, because no rule can read it. That finding has no repair.

- `repo/binary`: a file git tracks that opens with an ELF, Mach-O or PE/COFF magic number. `fix` deletes it, because a build makes it from source. Git still holds it. A tree that git cannot list is read from disk. The rule reads the blob git stores. A Git LFS file is never reported, because git holds its pointer and only the checkout holds the binary.
- `repo/near-duplicate`: a file whose lines match another file of its base name at `NearDuplicateShare` or above. The score is the Dice coefficient over non-blank trimmed lines, so a copy that differs in comments alone still trips. The later path of the pair is reported. A file each directory needs, such as `package.json`, `ts0.json`, a `justfile` or `Dockerfile`, is never compared. No attribute or list exempts a copy. The answer is one file that both places use. A symlink is that one file, and is never compared.

Both rules fetch a remote schema, once for each walk, and an XSD's imports with it. A schema that does not load, by a network error or any status but OK, is a finding. It is never a pass. The JSON Schema meta-schemas need no fetch, because the validator carries them.

The document rules walk what the other repository rules walk, so `testdata`, `node_modules`, a submodule and a nested clone stay out. `repo/binary` reads every tracked file, the large ones included.

A body that `AGENTS.md` already holds is not appended again. The import line is never copied into the file it imports. A `CLAUDE.md` that is a symlink stays.

Every other markdown file is left alone. The budget covers only the files that every request loads.
