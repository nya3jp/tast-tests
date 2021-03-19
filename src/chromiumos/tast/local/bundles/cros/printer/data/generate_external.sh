#!/bin/bash

# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script uploads files to the test server and generates corresponding
# external files that can be used by the printer Tast tests.

# Usage: ./generate_external.sh !(*.external|*.sh)
# Uploads all non-external files and generates corresponding external files.

# Remember to remove non-external files when finished: rm !(*.external|*.sh).

U="gs://chromiumos-test-assets-public/tast/cros/printer/"
#U="gs://chromeos-test-assets-private/tast/crosint/printer/"
D="$(date +"%Y%m%d")"
for A in "$@"; do
F="${A##*/}"
B="${U}${F%%.*}_${D}"
if [[ "${F}" == *.* ]]; then
  B="${B}.${F#*.}"
fi
gsutil cp "${A}" "${B}"
cat << EOF > "${F}.external"
{
  "url": "${B}",
  "size": $(stat -c %s "${A}"),
  "sha256sum": "$(sha256sum "${A}" | cut -d ' ' -f 1)"
}
EOF
done
