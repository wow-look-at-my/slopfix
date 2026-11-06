// Under --strict the lint errors of the build phases fail the run (exit 2),
// the convention of the suite.
int runSuite() {
	int failed = lint();
	return failed;
}
