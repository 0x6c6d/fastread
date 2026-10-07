#!/usr/bin/env bash
# Verification for T012 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/_lib.sh"
expect_ok "first line is 'MIT License', copyright line exact" <<'CMD'
head -1 LICENSE | grep -qx 'MIT License' &&
grep -qx 'Copyright (c) 2026 Lucas Menke' LICENSE
CMD
expect_ok "standard MIT text present" <<'CMD'
for p in 'Permission is hereby granted, free of charge, to any person obtaining a copy' \
  'of this software and associated documentation files (the "Software"), to deal' \
  'The above copyright notice and this permission notice shall be included in all' \
  'THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR' \
  'OTHER DEALINGS IN THE SOFTWARE.'; do
  grep -qF -- "$p" LICENSE || { echo "missing: $p"; exit 1; }
done
CMD
expect_fail "negative: other licence family or template placeholder in LICENSE" <<'CMD'
test -f LICENSE || exit 0
grep -n -i -e 'GNU General Public' -e 'Apache License' -e 'Redistribution and use in source' -e '\[year\]' -e '\[fullname\]' -e '<year>' -e '<copyright holders>' LICENSE
CMD
finish
