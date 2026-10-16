# gitread: the repository, read in-process

`gitread` answers the rules' questions about a repository from the files git writes. No git process starts for a read. A walk asks many questions, and each one as a subprocess costs a fork and an exec.

## What it reads

- The index, with the stat data of each entry.
- Loose objects, and packs through their index. A pack and its index are mapped once. A lookup searches the index in place through its fanout table.
- Delta chains. Each resolved base goes into a cache with a byte budget, the same budget git gives its own.
- Refs, packed refs, `FETCH_HEAD`, and revisions with `~`, `^` and `^{type}` suffixes and abbreviated names.
- Config: the system, global, repository and work tree files, `include.path`, and `GIT_CONFIG_COUNT`.
- `.gitignore`, `info/exclude`, `core.excludesFile`, `.gitattributes`, `info/attributes` and `core.attributesFile`. Each directory's file loads when a path under it is first asked about. Patterns match through a port of git's `wildmatch`.
- The environment overrides a hook sees: `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_COMMON_DIR` and `GIT_OBJECT_DIRECTORY`.

## What stays the same across callers

`Open` answers one `Repo` per repository for the whole process. A second caller finds what the first one read. A long-lived process, such as the language server, sees each change on disk: the index, the packed refs, the shallow file and each rule file are read again when their stamp moves. A pack directory is scanned again on a miss. A repository created again at the same path opens fresh.

## How it finds a change

`Changes` names what `git diff --name-only BASE` and `git ls-files --others --exclude-standard` name. A file whose stat data matches its index entry keeps the index's object name, as git trusts it. A file written in the same instant as the index is hashed, because its stat data cannot prove it is clean.

The merge base paints down from both sides by commit date, as git does. A walk stops once every queued commit is stale.

## What goes to git

- Network transport: `fetch` and `ls-remote`. A fetch writes the objects into the store, so a cached fork record can be checked on the next run.
- A blob that a partial clone left on its promisor remote. The fork scope fetches every missing blob in one call, the call git makes for a lazy fetch.
- Refs of a reftable repository. Objects there still read in-process.
- A revision spelling this package does not parse, such as a reflog entry.

A repository in the SHA-256 object format, a split index or a sparse index is an error that names the format.

## Tests

`gitread_test.go` checks each answer against git itself: the index, every object of a repository packed with offset deltas and with ref deltas, ignore and attribute rules, the file listing, the changed paths, submodules and revisions. `bench_test.go` times each read beside the git process that answered it before.
