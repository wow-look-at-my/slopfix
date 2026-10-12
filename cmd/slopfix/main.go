package main

import "github.com/wow-look-at-my/slopfix/cmd"

// go-toolchain fills both at link time: releaseNumber with the buildhost release
// (docs/RELEASE-STAMP.md there) and gitHash with the commit (docs/VCS-STAMP.md).
var (
	releaseNumber string
	gitHash       string
)

func main() {
	cmd.SetBuild(releaseNumber, gitHash)
	cmd.Execute()
}
