#!/bin/bash

# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script uploads files to the test server, generates corresponding
# external files that can be used by the printer Tast tests, and deletes
# the original files when finished.

# Usage: ./generate_external.sh !(*.external|*.sh)
# Uploads all non-external files, generates corresponding external files,
# and deletes the original files once finished.

die() {
  echo "$(basename "${0}"): error: $*" >&2
  exit 1
}

URL="gs://chromiumos-test-assets-public/tast/cros/printer/"
DATE="$(date +"%Y%m%d")"
for SRC in "$@"; do
  test -r "${SRC}" || die "could not read file: ${SRC}"
  FILE="$(basename "${SRC}")"
  DST="${URL}${FILE%%.*}_${DATE}"
  if [[ "${FILE}" == *.* ]]; then
    DST="${DST}.${FILE#*.}"
  fi
  cat << EOF > "${FILE}.external" || die "could not write file: ${FILE}.external"
{
  "url": "${DST}",
  "size": $(stat -c %s "${SRC}"),
  "sha256sum": "$(sha256sum "${SRC}" | cut -d ' ' -f 1)"
}
EOF
  gsutil cp "${SRC}" "${DST}" || die "gsutil failed: cp ${SRC} ${DST}"
  rm "${SRC}"
done
