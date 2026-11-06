import path from 'node:path';

/** Absolute path of the exporter under test. */
// The path.resolve call (not `new URL(..., import.meta.url)`) avoids the shadowed URL global of the jsdom test environment.
// Node's fileURLToPath rejects cross-realm URLs.
// A file path avoids both problems.
export const EXPORTER_MODULE = path.resolve(
	import.meta.dirname,
	'../../src/exporter.user.ts',
);
