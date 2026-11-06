set -o pipefail
sed -n 100,150p internal/trash/freedesktop_test.go
git show 4ecc41f:internal/trash/freedesktop.go | grep -n "dispose" | head