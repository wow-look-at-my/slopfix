# ratchet

The default branch judgs a branch. On CI, slopfix checks out the default branch, reads the command its `.github/ratchet` names there. And runs that command from the default branch's checkout with this branch's checkout as its last argument. A non-zero exit fails the build.

The command and everything it reads come from the default branch, so a branch cannot change what judges it. The word covers both halves the org used to spell separately: the runner that resolves the default branch. It runs the command (`ratchet.Check`), and the judge a repository points the file at (`cmd/ratchet`).

## The runner

`slopfix ratchet` is the entry point. It reads `CI` and returns at once when the variable is empty, so a local run never reaches the network. Outside a repository it returns too, because there is no branch to compare. It resolves the default branch with `git ls-remote --symref origin HEAD`, fetches it to a depth of one, and checks it out into a temporary worktree. A default branch whose `.github/ratchet` is absent judges nothing.

The command is read off the first line of the file that is neither blank nor a comment. A file that names no command is an error, not a pass. The command then runs from the default branch's checkout with this branch's root as its last argument. The worktree is removed afterwards.

A checkout holds no generated file. The runner runs `go mod tidy` and `go generate ./...` there before the command. Without that step a binary built from the checkout will run without its parse tables. The rules that read them will pass by finding nothing.

## The judge

`cmd/ratchet` is what slopfix's own `.github/ratchet` names. It builds binaries: the default branch's `slopfix` (its `check` is the verdict) and the branch's `slopfix` (its `fix` writes what the verdict judges). It materializes every case the default branch registers, repairs each with the branch's fix, and judges the result with the default branch's check. A case the default branch's check still flags is a failure.

That is what holds a branch to the guarantee. A rule it drops, a cap it loosens. A detection it narrows, or a repair it stops making leaves text the default branch's check still flags. A head that cannot be started is a failure, not a pass.

## Files

| Path | Role |
|---|---|
| `ratchet/run.go` | the runner: resolve the default branch, run its command |
| `ratchet/ratchet.go` | the judge: pair the two binaries and run the cases |
| `cmd/ratchet/main.go` | the judge's entry point, named by `.github/ratchet` |
| `cmd/ratchet.go` | the `ratchet` command that runs the runner |
| `.github/ratchet` | this repository's entry point: `go run ./cmd/ratchet` |
