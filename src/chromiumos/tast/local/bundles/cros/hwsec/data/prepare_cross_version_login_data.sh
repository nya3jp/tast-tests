#!/bin/bash

# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This bash scripts is used for creating the cross-version login testing data.
# Given the target CrOS version, the script will download the image from
# google storage and create user account in the image. Then, copy and upload
# the data to google storage so that we could use it in cross-version login
# testing.

error() {
  echo "[Error] $(basename "$0"): $*" >&2
}

die() {
  error "$*"
  exit 1
}

usage() {
  die "Usage: $(basename "$0") <testing_rsa> <board> <version>"
}

KEY_FILE="$1"
BOARD="$2"
VERSION="$3"

HOST="127.0.0.1"
PORT="9222"

DATE="$(date +"%Y%m%d")"
BASE_DIR="$(dirname "${BASH_SOURCE[0]}")"
DATA_DIR="${BASE_DIR}/cross_version_login"
TMP_DIR="/tmp/cross_version_login"
IMAGE="${TMP_DIR}/chromiumos_test_image.bin"

test -n "${KEY_FILE}" || usage
test -f "${KEY_FILE}" || die "keyfile '${KEY_FILE}' does not exist"
test -n "${BOARD}" || usage
test -n "${VERSION}" || usage

# Generates the content of the external data file for tast-tests.
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

# Gets the image of target version from Google Cloud Storage.
fetch_image() {
  local compressed_image="${TMP_DIR}/image.tar.xz"
  local gs_url="gs://chromeos-image-archive/${BOARD}-release/${VERSION}/chromiumos_test_image.tar.xz"
  if ! gsutil cp "${gs_url}" "${compressed_image}"; then
    error "gsutil failed to cp '${gs_url}' '${compressed_image}'"
    return 1
  fi
  if ! tar Jxvf "${compressed_image}" -C "${TMP_DIR}" ; then
    error "failed to decompress the '${compressed_image}'"
    return 1
  fi
  return 0
}

# Uploads the data to the Google Cloud Storage and generate the external data
# file for tast-tests.
upload_data() {
  local prefix="${VERSION}_${BOARD}_${DATE}"

  local data_name="${prefix}_data.tar.xz"
  local data_path="${TMP_DIR}/${data_name}"
  local remote_data_path="${TMP_DIR}/data.tar.xz"

  local config_name="${prefix}_config.json"
  local config_path="${DATA_DIR}/${config_name}"
  local remote_config_path="${TMP_DIR}/config.json"

  local external_file="${DATA_DIR}/${data_name}.external"
  local gs_url="gs://chromiumos-test-assets-public/tast/cros/hwsec/cross_version_login/${data_name}"

  if ! scp -o StrictHostKeyChecking=no -i "${KEY_FILE}" -P "${PORT}" \
      "root@${HOST}:${remote_data_path}" "${data_path}"
  then
    error "failed to scp the file '${data_path}'"
    return 1
  fi

  if ! scp -o StrictHostKeyChecking=no -i "${KEY_FILE}" -P "${PORT}" \
      "root@${HOST}:${remote_config_path}" "${config_path}"
  then
    error "failed to scp the file '${config_path}'"
    return 1
  fi

  if ! generate_external_file "${gs_url}" "${data_path}" > "${external_file}"
  then
    error "failed to write the file '${external_file}'"
    return 1
  fi
  if ! gsutil cp "${data_path}" "${gs_url}" ; then
    error "gsutil failed to cp '${data_path}' '${gs_url}'"
    return 1
  fi
  return 0
}

# Creates and upload the data file for cross-version login testing.
prepare_data() (
  local test_name="hwsec.PrepareCrossVersionLoginData"
  # "tpm2_simulator" is added by crrev.com/c/3312977, so this test cannot run
  # on older version. Therefore, adds -extrauseflags "tpm2_simulator" here.
  if ! tast run -failfortests -extrauseflags "tpm2_simulator" \
      "${HOST}:${PORT}" "${test_name}" ; then
    error "tast failed to run ${test_name}"
    return 1
  fi
  upload_data
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
