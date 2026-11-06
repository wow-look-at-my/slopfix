function commentOnlyLines(script: string): number[] {
	const scanner = ts.createScanner(ts.ScriptTarget.Latest, /* skipTrivia */ false, ts.LanguageVariant.Standard, script);
}
