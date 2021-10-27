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

KEY_FILE="$1"
BOARD="$2"
VERSION="$3"

HOST="127.0.0.1"
PORT="9222"

DATE="$(date +"%Y%m%d")"
DATA_DIR="$(dirname "${BASH_SOURCE[0]}")"
TMP_DIR="/tmp/cross_version_login"
IMAGE="${TMP_DIR}/chromiumos_test_image.bin"

test -f "${KEY_FILE}" || die "${KEY_FILE} does not exist"
test -n "${BOARD}" || die "missing board"
test -n "${VERSION}" || die "missing version"

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
  local compressed_image="${TMP_DIR}/image.tar.xz"
  local gs_url="gs://chromeos-image-archive/${BOARD}-release/${VERSION}/chromiumos_test_image.tar.xz"
  if ! gsutil cp "${gs_url}" "${compressed_image}"; then
    error "gsutil failed to cp ${gs_url} ${compressed_image}"
    return 1
  fi
  if ! tar Jxvf "${compressed_image}" -C "${TMP_DIR}" ; then
    error "failed to decompress the ${compressed_image}"
    return 1
  fi
  return 0
}

upload_data() {
  local file_on_dut="/tmp/cross_version_testing_data.tar.xz"
  local filename="data_${BOARD}_${VERSION}_${DATE}.tar.xz"
  local filepath="${TMP_DIR}/${filename}"
  local external_file="${DATA_DIR}/${filename}.external"
  local gs_url="gs://chromiumos-test-assets-public/tast/cros/hwsec/cross_version_login/${filename}"

  if ! scp -o StrictHostKeyChecking=no -i "${KEY_FILE}" -P "${PORT}" \
      "root@${HOST}:${file_on_dut}" "${filepath}"
  then
    error "failed to scp the file ${filepath}"
    return 1
  fi

  if ! generate_external_file "${gs_url}" "${filepath}" > "${external_file}"
  then
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
  if ! tast run "${HOST}:${PORT}" "${test_name}" ; then
    error "tast failed to run ${test_name}"
    return 1
  fi
  upload_data
  return $?
)


mkdir -p "${TMP_DIR}" || die "failed to mkdir ${TMP_DIR}"
if fetch_image ; then
  if cros_vm --log-level=warning --start --image-path="${IMAGE}" \
               --board="${BOARD}" ;
  then
    prepare_data
    cros_vm --log-level=warning --stop
  else
    error "cros_vm failed to start"
  fi
fi
rm -rf "${TMP_DIR}"
