#!/bin/bash
# Copyright 2022 The ChromiumOS Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script is meant to be run as PID 1 inside a VM.

# Do not set 'e' flag here as we expected some commands to return error
set -x

die() {
  echo "$1"
  exit 1
}

usage() {
  die "Usage: $(basename "$0")"
}

run_aplay() {
  aplay -D "hw:0,0" -d 10 --fatal-errors -c 2 -f S16_LE -r 48000 /dev/zero
}

main() {
  [[ "${BASHPID}" == "1" ]] || die "Not running as PID 1"

  [[ $# -eq 1 ]] || \
      die "Usage: $(basename "$0") <output_log>"

  local output_log="$1"

  {
    # `restart cras` while this command is running
    run_aplay
    echo "aplay 1 returned $?"

    # Sleep until cras is back
    sleep 5

    # this command should work without error
    run_aplay
    echo "aplay 2 returned $?"
  } &> "${output_log}"
}

main "$@"
exec poweroff -f
