# exclude.txt - paths left out of the snapshot.
#
# Read by a script after it stages the working set. One entry per
# line; blank lines and #-comments ignored.
#   dir/           - path PREFIX from the repo root (the whole directory)
#   **/name/       - a directory with that NAME at any depth

# Tool configuration.
.tools/
**/.tools/

# Tool docs.
NOTES.md
**/NOTES.md
