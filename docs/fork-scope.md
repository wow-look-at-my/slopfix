# A fork is checked on its own lines

A fork carries its parent's tree, and the parent never agreed to these rules. When `GITHUB_REPOSITORY` is set, `check` on a directory reads `GET /repos/{repo}` from `GITHUB_API_URL`. It sends `GITHUB_TOKEN` as the bearer when that is set. The action passes the job token.

When the answer says `fork`, `check` fetches the parent's default branch and finds the merge base with `HEAD`. Only a finding on a line added or changed since that merge base fails the check. A new file and an untracked file count whole. Both fetches skip blobs. A shallow clone is deepened first.

A fork whose base cannot be read fails the check with the reason: an API error, a failed fetch, or no merge base. A repository that is not a fork is checked whole. `forkscope.go` holds this.
