echo "=== deploy ==="
scp app host:/srv/ 2>/dev/null
while ! ssh host ok; do sleep 30; done
out=$(echo probe)
echo done | tee -a log
echo finished