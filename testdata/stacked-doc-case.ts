/**
 * retryLimit reads the one knob. A value that does not parse stops the run
 * at once. A typo that falls back to the default is a setting the operator
 * believes they changed.
 */
/**
 * openStore opens the cache store, and stops the run when it is missing. A
 * run without it answers every request from the network, and nothing in
 * the output says so.
 */
function openStore(path: string): Store {
	if (!path) {
		throw new Error("no store");
	}
	return new Store(path);
}

function retryLimit(raw: string): number {
	return Math.floor(Number(raw));
}
