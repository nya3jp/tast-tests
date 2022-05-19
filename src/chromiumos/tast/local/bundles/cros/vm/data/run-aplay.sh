#!/bin/bash
# Copyright 2022 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script is meant to be run as PID 1 inside a VM.
set -ex

die() {
  echo "$1"
  exit 1
}

usage() {
  die "Usage: $(basename "$0")"
}

main() {
  [[ "${BASHPID}" == "1" ]] || die "Not runnnig as PID 1"

  [[ $# -eq 1 ]] || \
      die "Usage: $(basename "$0") <output_log>"

  local output_log="$1"

  aplay -l > "${output_log}"
}

main "$@"
exec poweroff -f
