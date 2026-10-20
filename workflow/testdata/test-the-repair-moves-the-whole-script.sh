      - run: |
          npm ci
          if [ ! -f out.txt ]; then
            echo "::error::no out.txt"; exit 1
          fi
      - run: npm test
