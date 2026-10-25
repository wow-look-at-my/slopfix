/** ollamaLibraryRecords is what ingest reads: `family name -> record text`,
 * the same pair shape `splitTopLevel` yields for every other source. `mode` is
 * written ONLY where a pill states one, which today means only `embedding`.
 * This source is the lowest-priority one for every `ollama/*` key, so anything
 * it writes here wins outright. A `chat` it merely assumed would beat
 * litellm's stated `completion` on those ollama base models that really do
 * complete rather than converse. It knows one thing the others do not; writing
 * more than that turns a correction into a second blanket label. What a family
 * with no such pill IS, is decided in `ingest.ts` (`modeFor`). */
export function* ollamaLibraryRecords(html: string): Generator<[string, string]> {
	for (const family of ollamaFamilies(html)) {
		const record: Record<string, unknown> = { provider: "ollama" };
		const mode = statedMode(family.capabilities);
		if (mode) record.mode = mode;
		record.capabilities = family.capabilities;
		record.sizes = family.sizes;
		if (family.description) record.description = family.description;
		yield [family.name, JSON.stringify(record)];
	}
}
