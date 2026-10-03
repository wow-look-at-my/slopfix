# A fork is checked on its own lines

A fork carries its parent's tree, and the parent never agreed to these rules.

A fork that GitHub does not record as one names its upstream in `.github/fork-of`. The first line that is not blank and not a `#` comment is the upstream URL. `check` lists the upstream's tags with `git ls-remote`. The base is the newest commit of HEAD's history that a tag names. For a fork that merges upstream releases, that is the latest release it merged. Only a finding on a line changed since that base fails the check. The file wins over the API. It works outside GitHub Actions. A file with no URL fails the check with the reason. So does an upstream that cannot be listed or has no tags, and a HEAD that contains none of them.

Without that file, when `GITHUB_REPOSITORY` is set, `check` on a directory reads `GET /repos/{repo}` from `GITHUB_API_URL`. It sends `GITHUB_TOKEN` as the bearer when that is set. The action passes the job token.

When the answer says `fork`, `check` fetches the parent's default branch and finds the merge base with `HEAD`. Only a finding on a line added or changed since that merge base fails the check. A new file and an untracked file count whole. Both fetches skip blobs. A shallow clone is deepened first.

A fork whose base cannot be read fails the check with the reason: an API error, a failed fetch, or no merge base. A repository that is not a fork is checked whole. `forkscope.go` holds this.
