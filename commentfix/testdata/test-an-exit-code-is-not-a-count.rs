/// Dispatch the observe-only session-end `Stop`: runs in stop-gate mode so
/// exit code 2 parses as a block, but the decision is discarded (no turn
/// left to continue).
pub(crate) async fn dispatch_session_end_stop(&self, reason: &str) {
    if self.startup_hints.is_subagent {
        return;
    }
}
