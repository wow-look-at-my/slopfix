# A fork is checked on its own lines

A fork carries its parent's tree, and the parent never agreed to these rules.

The org's `.github` repository lists each fork that GitHub does not record, in `fork-list/fork-of.json`. A repository therefore cannot declare itself a fork to escape the rules. The list is a JSON object, comments allowed, that maps each `OWNER/NAME` to its upstream URL. A name matches without regard to case. `forklist.schema.json` holds the shape, and json-validator checks the list against it, both here and in that repository's CI before the publish. That repository's CI publishes the directory as the buildhost site of project `github`. When `GITHUB_REPOSITORY` is set, `check` reads the list from `ForkListURL`, a cached CDN read. This is because GitHub drops requests at the rate that every CI run makes. No variable moves the URL, because a workflow sets its own variables. When the list names the repository, `check` lists the upstream's tags with `git ls-remote`. The base is the newest commit of HEAD's history that a tag names. For a fork that merges upstream releases, that is the latest release it merged. Only a finding on a line changed since that base fails the check. An org with no list, and a repository the list does not name, fall through to the API. A list that cannot be read or has a malformed line fails the check with the reason. So does an upstream that cannot be listed or has no tags, and a HEAD that contains none of them.

Otherwise, when `GITHUB_REPOSITORY` is set, `check` on a directory reads `GET /repos/{repo}` from `GITHUB_API_URL`. It sends `GITHUB_TOKEN` as the bearer when that is set. The action passes the job token.

When the answer says `fork`, `check` fetches the parent's default branch and finds the merge base with `HEAD`. Only a finding on a line added or changed since that merge base fails the check. A new file and an untracked file count whole. Both fetches skip blobs. A shallow clone is deepened first.

A fork whose base cannot be read fails the check with the reason: an API error, a failed fetch, or no merge base. A repository that is not a fork is checked whole. The `forkscope` package holds this.

## Which repository

The repository is the one the work tree's `origin` names on the GitHub server, `GITHUB_SERVER_URL` or github.com. So a run on a developer's machine finds the fork too. With no such origin, it is `GITHUB_REPOSITORY`, for a root inside `GITHUB_WORKSPACE`. An origin elsewhere and no `GITHUB_REPOSITORY` mean no fork, and no request goes out. When the Actions event payload says the repository is no fork, the API is not asked.

What the list, the API and the fetch said is kept in `slopfix-fork.json` in the git common directory for an hour. The write hook runs on every write and asks the network once.

## Every write keeps to the fork's lines

- `slopfix fix .` and `check .` skip each file the fork never touched: it is neither read nor written. In a file the fork touched, a repair lands only on a line the fork wrote. A finding counts only on such a line. A repository rule rewrites or deletes only a file the fork wrote whole, and reports the rest.
- `slopfix fix FILE` on a file the fork never touched does nothing. On a file it touched, only the fork's lines change.
- The write hook judges the text as it will land. A repair lands only on a line that differs from the base once the write lands. An edit to an inherited line that `slopfix fix` will repair is not refused, because `slopfix fix` leaves that line alone.
- `commentfix.FixTree`, which go-toolchain runs on every build, keeps to the fork's lines the same way.

A change is a run of lines a line diff pairs up. It lands when the fork wrote every line it replaces. A run of new lines lands when the fork wrote a line beside it. A fork whose base cannot be read writes nothing: the run fails, and the hook refuses the write.
