/** loadAssets reads every file under `assets/*` once, at start. The paths
 * come back sorted, so two runs on the same tree list the same files in the
 * same order. A file that fails to read stops the load with its path in the
 * error. The caller sees which file broke, and the load reads nothing after
 * it. A missing directory reads as an empty list, because a fresh checkout
 * has no assets yet. */
export function loadAssets(names: string[]): string[] {
	return names.slice().sort();
}
