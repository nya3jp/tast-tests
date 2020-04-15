#!/usr/bin/env bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x

readonly CRYPTO_HDR=/tmp/crypto_tmp/crypthdr.img
readonly RAM_DEV=/dev/ram0
readonly CRYPTO_DEV_NAME=encrypted-ram0
readonly CRYPTO_DEV=/dev/mapper/${CRYPTO_DEV_NAME}
readonly RESULTS_DIR=$1/results/`uname -r`
readonly PASSPHRASE=123
readonly LOOP_COUNT=1000

rm -rf "${RESULTS_DIR}"
mkdir -p "${RESULTS_DIR}"

for test_type in randrw readwrite; do
for blocksize in 512 1k 2k 4k 8k 16k 32k 64k 128k 256k 512k 1m; do

# No optimization flags.

echo -n "${PASSPHRASE}" | cryptsetup open \
    --header "${CRYPTO_HDR}" \
    "${RAM_DEV}" "${CRYPTO_DEV_NAME}" -

fio \
    --filename="${CRYPTO_DEV}" \
    --readwrite="${test_type}" \
    --bs="${blocksize}" \
    --direct=1 \
    --loops="${LOOP_COUNT}" \
    --name="crypt-${test_type}-${blocksize}" \
    --output "${RESULTS_DIR}/crypt_no_opt-${test_type}-${blocksize}" \
    --output-format=json

cryptsetup close "${CRYPTO_DEV_NAME}"

# With --sam_cpu_crypt

echo -n "${PASSPHRASE}" | cryptsetup open \
    --header "${CRYPTO_HDR}" \
    --perf-same_cpu_crypt \
    "${RAM_DEV}" "${CRYPTO_DEV_NAME}"

fio \
    --filename="${CRYPTO_DEV}" \
    --readwrite="${test_type}" \
    --bs="${blocksize}" \
    --direct=1 \
    --loops="${LOOP_COUNT}" \
    --name="same_cpu_crypt-${test_type}-${blocksize}" \
    --output "${RESULTS_DIR}/same_cpu_crypt-${test_type}-${blocksize}" \
    --output-format=json

cryptsetup close "${CRYPTO_DEV_NAME}"

# With --submit_from_crypt_cpus

echo -n "${PASSPHRASE}" | cryptsetup open \
    --header "${CRYPTO_HDR}" \
    --perf-submit_from_crypt_cpus \
    "${RAM_DEV}" "${CRYPTO_DEV_NAME}"

fio \
    --filename="${CRYPTO_DEV}" \
    --readwrite="${test_type}" \
    --bs="${blocksize}" \
    --direct=1 \
    --loops="${LOOP_COUNT}" \
    --name="submit_from_crypt-${test_type}-${blocksize}" \
    --output "${RESULTS_DIR}/submit_from_crypt-${test_type}-${blocksize}" \
    --output-format=json

cryptsetup close "${CRYPTO_DEV_NAME}"

# With both flags.

echo -n "${PASSPHRASE}" | cryptsetup open \
    --header "${CRYPTO_HDR}" \
    --perf-same_cpu_crypt \
    --perf-submit_from_crypt_cpus \
    "${RAM_DEV}" "${CRYPTO_DEV_NAME}"

fio \
    --filename="${CRYPTO_DEV}" \
    --readwrite="${test_type}" \
    --bs="${blocksize}" \
    --direct=1 \
    --loops="${LOOP_COUNT}" \
    --name="both-${test_type}-${blocksize}" \
    --output "${RESULTS_DIR}/both-${test_type}-${blocksize}" \
    --output-format=json

cryptsetup close "${CRYPTO_DEV_NAME}"

# Non-encrypted device.

fio \
    --filename="${RAM_DEV}" \
    --readwrite="${test_type}" \
    --bs="${blocksize}" \
    --direct=1 \
    --loops="${LOOP_COUNT}" \
    --name="plain-${test_type}-${blocksize}" \
    --output "${RESULTS_DIR}/plain-${test_type}-${blocksize}" \
    --output-format=json

done
done
