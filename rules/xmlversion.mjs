// xmlversion rewrites the XML declaration of every file named on the command
// line to 1.1, which is the version this repository's rules are written in.
import { readFileSync, writeFileSync } from "node:fs";

for (const path of process.argv.slice(2)) {
	const before = readFileSync(path, "utf8");
	const after = before.replace(
		/^<\?xml version="1\.0"/,
		'<?xml version="1.1"',
	);
	if (after === before) continue;
	writeFileSync(path, after);
	console.log(path);
}
