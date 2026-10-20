fn config() {
    // 2) Every axis overridden -> the struct round-ends and the emitted rules
    //    on this platform reflect a read-only grok home / writable system base.
    let over = "[jail]\ncwd = \"ro\"\ngrok_home = \"ro\"\ntmp = \"rw\"\nsystem = \"rw\"\n";
}
