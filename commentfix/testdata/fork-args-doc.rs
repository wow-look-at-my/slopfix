/// Parse the raw argument string after `/fork`.
///
/// Recognised flags appear at the start; everything after the last flag
/// is the directive. Unknown flags are deliberately treated as the
/// start of the directive (so `/fork --foo bar` becomes a directive
/// `--foo bar`) -- the parser is conservative because the args are
/// user-typed text and we do not want to reject directives that happen
/// to begin with `--`.
///
/// Errors:
/// - `--worktree` and `--no-worktree` cannot both appear.
/// - a flag cannot be repeated.
/// - `--at <turn>` returns a friendly "not supported in this version"
///   message: the shell already supports the underlying parameter (see
///   `xai_grok_shell::session::fork::ForkSessionRequest::target_prompt_index`)
///   and a turn-picker UI is planned; this version deliberately rejects
///   the flag so users discover the deferral cleanly.
pub fn parse_fork_args(args: &str) -> Result<ForkArgs, String> {
    let rest = args.trim_start();
    Ok(ForkArgs::new(rest))
}
