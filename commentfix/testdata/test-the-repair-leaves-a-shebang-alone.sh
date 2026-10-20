#!/usr/bin/env bash
# Provisions bubblewrap on a Linux runner. Two phases need it. The dats phase
# sandboxes every suite command. The go command confines the generate directive
# of every dependency that carries one, and stops the build when it cannot.
#
# So every Linux build needs a backend, not only a module with dats suites: a
# module's own tree says nothing about what its dependencies generate.
#
# usage: provision-bwrap.sh
set -euo pipefail
