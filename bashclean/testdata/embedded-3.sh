set -o pipefail
scp app host:/srv/
while ! ssh host ok; do sleep 3; done
out=$(echo probe)
echo done | tee -a log