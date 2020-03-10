#!/bin/bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script is meant to be run as PID 1 inside a VM.
set -e -o pipefail

die() {
  echo "$1"
  exit 1
}

usage() {
  die "Usage: $(basename "$0") <path_to_test_directory>"
}

main() {
  local testdir="$1"
  local encrypted="${testdir}/encrypted"

  [[ "$$" == "1" ]] || die "Not running as PID 1"

  [[ $# -eq 1 ]] || usage

  [[ -d "${testdir}" ]] || die "${testdir} is not a directory"

  [[ -d "${encrypted}" ]] ||  die "No encrypted directory in ${testdir}"

  # We are running as pid 1.  Mount some necessary file systems.
  mount -t proc proc /proc
  mount -t sysfs sys /sys
  mount -t tmpfs tmp /tmp
  mount -t tmpfs run /run

  local key
  key="$(e4crypt get_policy "${encrypted}" | awk '{ print $2 }')"

  echo "Encrypted directory key: ${key}"

  local newdir="${testdir}/newdir"

  mkdir "${newdir}"
  e4crypt set_policy "${key}" "${newdir}"
}

main "$@"
