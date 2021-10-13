#!/usr/bin/env bash

# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Tool for generating or updating *.external files for use with tast.
# Run with no arguments for usage examples.

print_usage() {
  date=$(date +"%Y%m%d")
  cat <<- EOM
Generates or updates an external file

Example usage:

external_file_uploader.sh local_file.json  gs://bucket/path/

  Uploads local_file.json to gs://bucket/path/local_file_${date}.json

  The remote object file name is based on the local file name. A timestamp is
  inserted into the file name based on today's date.

  This also generates a new file, local_file.json.external, that links to the
  new remote object.

external_file_uploader.sh local_file.json  gs://bucket/path/remote.json

  Uploads local_file.json to gs://bucket/path/remote_${date}.json

  A timestamp is inserted into the remote object name file name based on
  today's date.

  This also generates a new file, remote.json.external, that links to the new
  remote object.


external_file_uploader.sh local_file.json remote.json.external

  Uploads local_file.json to gs://bucket/path/remote_${date}.json where
  gs://bucket/path/ is inferred from reading the URL in remote.json.external.
  The remote object file name is based on the *.external file name.

  A timestamp is inserted into the file name to avoid collisions with previous
  files which may be in use by other tests.

  This also re-generates remote.json.external so it links to the new remote
  object.
EOM
}


if [ $# -ne 2 ]; then
  print_usage
  exit 1
fi

# Check requirements
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

source_file="$1"
if [ ! -f "${source_file}" ]; then
  echo source file not found
  return 1;
fi

target=$2
file_name=$(basename "${source_file}")
date=$(date +"%Y%m%d")

case "$target" in
*/)
  # Target is a bucket. Use the source file's file name.
  # A new .external file will be generated at ./$(file_name).external.
  target_bucket=${target}
  external_file=${file_name}.external
  ;;
*.external)
  # Target is an existing .external file. Use the .external file's name as the
  # remote file name.  The .external file will be re-generated with the new
  # size, checksum and URL
  file_name=$(echo ${target} | sed 's/\.external//')
  # Get the target bucket and directory from the .external file
  target_bucket=$(dirname "$(cat "${target}" | jq -r '.url')")"/"
  external_file=${target}
  ;;
*)
  # User specified a full file name for the target. Use the target's file name
  # to name the remote file. A new .external file will be generated at
  # ./$(file_name).external.
  file_name=$(basename "${target}")
  target_bucket=$(dirname "${target}")"/"
  external_file=${file_name}.external
  ;;
esac

# Insert a timestamp in front of the first "." in the remote file path. If
# there are no "."'s in the file name, append the date
target_file_name_with_date=$(echo ${file_name} |
                             sed 's/\./_'${date}'&/; t; s/$/_'${date}'/')
target=${target_bucket}${target_file_name_with_date}

gsutil.py -q cp "${source_file}" "${target}"

size=$(du -b "${source_file}" | cut -f 1)
sha=$(sha256sum --binary ${source_file} | cut -d ' ' -f 1)

# Generate a new .external file. Use "jq ." to clean up the formatting.
echo '{"url": "'${target}'", "size": '${size}', "sha256sum": "'${sha}'"}' \
     | jq . > "${external_file}"

echo Created "${external_file}", which references "${target}".
