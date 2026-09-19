// truncate-number-comments.mjs deletes each comment line slopfix reports as
// stating a number. It reads `slopfix comments <path>...` output on stdin.
//
// Deleting the line is the repair: no rewrite turns a stated count into prose,
// and a comment that carries a count is worth less than no comment.
import { readFileSync, writeFileSync } from "node:fs";

const text = readFileSync(0, "utf8");
const byFile = new Map();
for (const line of text.split("\n")) {
	const m = /^([^\s:]+):(\d+):\d+: /.exec(line);
	if (!m) continue;
	const [, file, no] = m;
	if (!byFile.has(file)) byFile.set(file, new Set());
	byFile.get(file).add(Number(no) - 1);
}

for (const [file, drop] of byFile) {
	const lines = readFileSync(file, "utf8").split("\n");
	const kept = lines.filter((_, i) => !drop.has(i));
	writeFileSync(file, kept.join("\n"));
	console.log(`${file}: dropped ${drop.size}`);
}
