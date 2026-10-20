# go mod tidy keeps the placeholders too, and needs no go.sum entry for an org
# module. A second tidy leaves go.mod byte-identical.
cd $WORK/m
go mod tidy
grep '^	github.com/wow-look-at-my/alpha v0\.0\.0$' go.mod
grep '^	github.com/wow-look-at-my/beta v0\.0\.0$' go.mod
! exists go.sum
env GIT_AUTHOR_DATE=2026-02-03T04:05:06Z
exec git commit -am 'beta next'
