#!/usr/bin/env bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x

readonly CRYPTO_DIR=/tmp/crypto_tmp
readonly CRYPTO_HDR=/tmp/crypto_tmp/crypthdr.img
readonly RAM_DEV=/dev/ram0
readonly CRYPTO_DEV_NAME=encrypted-ram0
readonly CRYPTO_DEV=/dev/mapper/${CRYPTO_DEV_NAME}
readonly MASTER_KEY=/tmp/masterkey

rm -rf "${CRYPTO_DIR}"
mkdir -p "${CRYPTO_DIR}"
modprobe brd rd_nr=1 rd_size=4194304
dd if=/dev/zero of="${CRYPTO_HDR}" bs=2M count=1
echo -n "123" | cryptsetup luksFormat "${RAM_DEV}" --header "${CRYPTO_HDR}" -
