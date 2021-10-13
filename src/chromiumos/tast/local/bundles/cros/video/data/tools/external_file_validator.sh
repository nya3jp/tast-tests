#!/usr/bin/env bash

# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Tool for validating *.external files used by tast.

print_usage() {
  cat <<- EOM
external_file_verifier.sh <external_file>

Download's and validates the size and SHA of the remote file specified in the
.external file.
EOM
}


if [ $# -ne 1 ]; then
  print_usage
  exit 1
fi

external_file="$1"
if [ ! -f "${external_file}" ]; then
  echo File \"${external_file}\" was not found
  exit 1
fi

# Check external requirements
if ! command -v jq &> /dev/null; then
  echo jq is required, but could not be found. See https://stedolan.github.io/j\
q/download/ for installation steps.
  exit 1
fi

if ! command -v gsutil.py &> /dev/null; then
    echo "gsutil.py is required, but could not be found. See https://commondat\
astorage.googleapis.com/chrome-infra-docs/flat/depot_tools/docs/html/depot_too\
ls_tutorial.html#_setting_up for installation steps."
    exit 1
fi

# Parse .external file
url=$(cat "$external_file" | jq -r '.url')

expected_size=$(cat "$external_file" | jq -r '.size')
expected_sha=$(cat "$external_file" | jq -r '.sha256sum')

# Download file to a temporary file and validate its size and checksum
tmpfile=$(mktemp)

echo Downloading remote file \"${url}\". This may take some time.
gsutil.py -q cp "${url}" "${tmpfile}"

echo Verifying downloaded file.

size=$(du -b "${tmpfile}" | cut -f 1)
sha=$(sha256sum --binary "${tmpfile}" | cut -d ' ' -f 1)
rm "${tmpfile}"

if [ ${size} != ${expected_size} ]; then
  echo The remote file \"${url}\" had an unexpected size ${size}.\
 Expected ${expected_size}.
  exit 1
fi

if [ ${sha} != ${expected_sha} ]; then
  echo The remote file \"${url}\" had an unexpected sha256 ${sha}.\
 Expected ${expected_sha}.
  exit 1
fi

echo \"$external_file\" is valid.
