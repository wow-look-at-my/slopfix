// resolve.js resolves each conflicted path to the version in a named commit,
// and refuses to leave a conflict marker behind. It is a one-off merge driver,
// run by hand and not by the build.
const { execFileSync } = require("child_process");
const fs = require("fs");

const ref = process.argv[2];
for (const path of process.argv.slice(3)) {
	const want = execFileSync("git", ["show", ref + ":" + path], { maxBuffer: 1 << 28 });
	if (/^<<<<<<< /m.test(want.toString())) {
		throw new Error(ref + " carries a conflict marker in " + path);
	}
	fs.writeFileSync(path, want);
	const back = fs.readFileSync(path, "utf8");
	if (/^<<<<<<< /m.test(back)) {
		throw new Error("still conflicted after the write: " + path);
	}
	console.log("resolved " + path);
}
