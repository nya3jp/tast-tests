#!/bin/bash

# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

die() {
  echo "$(basename "$0"): $*" >&2
  exit 1
}

host="$1"
board="$2"
version="$3"

filename="cross_version_login_data_${board}_${version}.tar.xz"
filepath="/tmp/${filename}"
gs_url="gs://chromiumos-test-assets-public/tast/cros/hwsec/${filename}"

scp "${host}":"/tmp/cross_version_testing_data.tar.xz" "${filepath}" || die "failed to scp the file ${filepath}"

cat <<EOF > "${filename}.external" || die "failed to write the file: ${filename}.external"
{
  "url": "${gs_url}",
  "size": $(stat -c %s "${filepath}"),
  "sha256sum": "$(sha256sum "${filepath}" | cut -d ' ' -f 1)"
}
EOF

gsutil cp "${filepath}" "${gs_url}" || die "gsutil failed to cp ${filepath} ${gs_url}"
rm $filepath

