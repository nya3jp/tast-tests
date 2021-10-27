#!/bin/bash

# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


error() {
  echo "[Error] $(basename "$0"): $*" >&2
}

die() {
  error "$*"
  exit 1
}

keyfile="$1"
board="$2"
version="$3"
#test -n "${host}" || die "missing host"
#test -n "${channel}" || die "missing channel"

host="127.0.0.1"
port="9222"

tmp_dir="/tmp/cross_version_login"
image="${tmp_dir}/chromiumos_test_image.bin"

test -f "${keyfile}" || die "${keyfile} does not exist"
test -n "${board}" || die "missing board"
test -n "${version}" || die "missing version"

generate_external_file() {
  local gs_url="$1"
  local filepath="$2"
  cat <<EOF
{
  "url": "${gs_url}",
  "size": $(stat -c %s "${filepath}"),
  "sha256sum": "$(sha256sum "${filepath}" | cut -d ' ' -f 1)"
}
EOF
}

fetch_image() {
  local compressed_image="${tmp_dir}/image.tar.xz"
  #local version_short="$(echo "${version}" | cut -d"-" -f2)"
  #local gs_url="gs://chromeos-releases/${channel}-channel/betty/${version_short}/ChromeOS-test-${version}-${board}.tar.xz"
  local gs_url="gs://chromeos-image-archive/${board}-release/${version}/chromiumos_test_image.tar.xz"
  if ! gsutil cp "${gs_url}" "${compressed_image}"; then
    error "gsutil failed to cp ${gs_url} ${compressed_image}"
    return 1
  fi
  if ! tar Jxvf "${compressed_image}" -C "${tmp_dir}" ; then
    error "failed to decompress the ${compressed_image}"
    return 1
  fi
  return 0
}

upload_data() {
  local data_dir="$(dirname ${BASH_SOURCE[0]})"
  local file_on_dut="/tmp/cross_version_testing_data.tar.xz"
  local filename="data_${board}_${version}.tar.xz"
  local filepath="${tmp_dir}/${filename}"
  local external_file="${data_dir}/${filename}.external"
  local gs_url="gs://chromiumos-test-assets-public/tast/cros/hwsec/cross_version_login/${filename}"
  scp -o StrictHostKeyChecking=no -i "${keyfile}" -P "${port}" \
      "root@${host}:${file_on_dut}" "${filepath}"
  if [[ $? -ne 0 ]] ; then
    error "failed to scp the file ${filepath}"
    return 1
  fi

  generate_external_file "${gs_url}" "${filepath}" > "${external_file}"
  if [[ $? -ne 0 ]] ; then
    error "failed to write the file: ${filename}.external"
    return 1
  fi
  if ! gsutil cp "${filepath}" "${gs_url}" ; then
    error "gsutil failed to cp ${filepath} ${gs_url}"
    return 1
  fi
}

prepare_data() (
  local test_name="hwsec.PrepareCrossVersionLoginData"
  if ! tast run "${host}:${port}" "${test_name}" ; then
    error "tast failed to run ${test_name}"
    return 1
  fi
  upload_data
  return $?
)


mkdir -p "${tmp_dir}" || die "failed to mkdir ${tmp_dir}"
if fetch_image ; then
  cros_vm --log-level=warning --start --image-path="${image}" --board="${board}"
  if [[ $? -eq 0 ]] ; then
    prepare_data
    cros_vm --log-level=warning --stop
  else
    error "cros_vm failed to start"
  fi
fi
rm -rf "${tmp_dir}"
