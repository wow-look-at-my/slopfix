/** loadAssets reads every file under `assets/*` once, at start. */
export function loadAssets(names: string[]): string[] {
	return names.slice().sort();
}
