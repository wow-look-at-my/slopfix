/** shelfRecords is what the loader reads: `shelf name -> record text`,
 * the same pair shape `splitPairs` yields for every other source. `kind` is
 * written ONLY where a label states one, which today means only `archive`.
 * This source is the lowest-priority one for every `shelf/*` key, so anything
 * it writes here wins outright. A `book` it merely guessed would beat
 * the catalog's stated `journal` on those shelf rows that really do
 * hold journals rather than books. It knows one thing the others do not; writing
 * more than that turns a correction into a second blanket label. What a shelf
 * with no such label IS, is decided in `shelf.ts` (`kindFor`). */
export function* shelfRecords(html: string): Generator<[string, string]> {
	for (const shelf of splitPairs(html)) {
		const record: Record<string, unknown> = { source: "shelf" };
		const kind = kindFor(shelf.labels);
		if (kind) record.kind = kind;
		record.labels = shelf.labels;
		record.sizes = shelf.sizes;
		if (shelf.note) record.note = shelf.note;
		yield [shelf.name, JSON.stringify(record)];
	}
}

/** splitPairs answers each shelf of the page. */
export function splitPairs(html: string): Shelf[] {
	return JSON.parse(html);
}

/** kindFor answers the kind a label states. */
export function kindFor(labels: string[]): string {
	return labels.includes("archive") ? "archive" : "";
}
