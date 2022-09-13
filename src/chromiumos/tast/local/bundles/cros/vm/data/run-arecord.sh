#!/bin/bash
# Copyright 2022 The ChromiumOS Authors
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
  [[ "$$" == "1" ]] || die "Not runnnig as PID 1"

  [[ $# -eq 1 ]] || \
      die "Usage: $(basename "$0") <output_path>"

  local output_path="$1"

  arecord -l > "${output_path}"
}

main "$@"
exec poweroff -f
