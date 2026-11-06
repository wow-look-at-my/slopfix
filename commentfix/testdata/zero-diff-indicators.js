const before = 1;

// The zero-diff indicators, written before the merge: the title gains ZERO_DIFF_TITLE_PREFIX and the
// description opens with ZERO_DIFF_BODY_NOTE. Each is added only when missing. A failed PATCH is loud
// and does not hold up the merge.
export const ZERO_DIFF_TITLE_PREFIX = '[zero diff]';
export const ZERO_DIFF_BODY_NOTE = '> **Zero diff:**';
async function stampZeroDiff(repo: string, pr: any, baseRef: string, headRef: string, token: string): Promise<void> {
  const title = String(pr.title ?? '');
  const body = String(pr.body ?? '');
  const titled = title.startsWith(ZERO_DIFF_TITLE_PREFIX);
  const noted = body.startsWith(ZERO_DIFF_BODY_NOTE);
  if (titled && noted) return;
  const note = `${ZERO_DIFF_BODY_NOTE} every change on ${headRef} is already on ${baseRef}`;
  const newTitle = titled ? title : `${ZERO_DIFF_TITLE_PREFIX} ${title || headRef}`;
  const newBody = noted ? body : body ? `${note}\n\n${body}` : note;
  await updatePullText(repo, pr.number, newTitle, newBody, token);
}
