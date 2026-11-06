          for f in artifacts/debugger/*.html; do
            if ! grep -q 'STOP_AT ==' "$f"; then
              echo "FAIL: no STOP_AT breakpoints in $f"
              exit 1
            fi
            echo "OK"
          done
