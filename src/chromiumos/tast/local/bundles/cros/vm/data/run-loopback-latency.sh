#!/bin/bash
# Copyright 2021 The ChromiumOS Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script is meant to be run as PID 1 inside a VM.
set -x

die() {
  echo "$1"
  exit 1
}

usage() {
  die "Usage: $(basename "$0")"
}

main() {
  [[ "$$" == "1" ]] || die "Not runnnig as PID 1"

  [[ $# -ge 3 ]] || \
      die "Usage: $(basename "$0") <loop> <log> <buffer_sizes>..."

  local loop="$1"
  local log="$2"
  local buffer_sizes=("${@:3}")

  for buffer_size in "${buffer_sizes[@]}"; do
    truncate -s 0 "${log}.${buffer_size}"
    for (( i = 0; i < loop; i++ )); do
      loopback_latency -i hw:0,0 -o hw:0,0 -n 1000 -r 48000 \
        -p $((buffer_size/2)) -b "${buffer_size}" &>> "${log}.${buffer_size}"
    done
  done
}

main "$@"
exec poweroff -f
