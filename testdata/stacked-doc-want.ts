/** ttlOf reads the one knob. An unparseable value FAILS rather than falling
 * back to the default. */
/** snapshotOf reads the bucket binding, and FAILS when it is missing. */
function snapshotOf(env: Env): R2Bucket {
	if (!env.SNAPSHOT) {
		throw new Error(
			"the SNAPSHOT R2 binding is missing; create the bucket and deploy with the binding in wrangler.toml",
		);
	}
	return env.SNAPSHOT;
}

function ttlOf(env: Env): number {
	const raw = env.MODELINFO_TTL_SECONDS?.trim();
	if (!raw) return TTL_SECONDS;
	const n = Number(raw);
	if (!Number.isFinite(n) || n <= 0) {
		throw new Error(
			`MODELINFO_TTL_SECONDS is ${JSON.stringify(raw)}, which is not a positive number of seconds`,
		);
	}
	return Math.floor(n);
}
