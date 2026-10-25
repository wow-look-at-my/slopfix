/** retryLimit reads the one knob. */
/** openStore opens the cache store, and stops the run when it is missing. */
function openStore(path: string): Store {
	if (!path) {
		throw new Error("no store");
	}
	return new Store(path);
}

function retryLimit(raw: string): number {
	return Math.floor(Number(raw));
}
