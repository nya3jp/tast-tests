#!/bin/bash
# Copyright 2021 The Chromium OS Authors. All rights reserved.
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

  [[ $# -eq 4 ]] || \
      die "Usage: $(basename "$0") <period_size> <buffer_size> <loop> <log>"

  local period_size="$1"
  local buffer_size="$2"
  local loop="$3"
  local log="$4"
  truncate -s 0 ${log}

  for (( i = 0; i < ${loop}; i++ )); do
    loopback_latency -i hw:0,0 -o hw:0,0 -n 1000 -r 48000 \
      -p ${period_size} -b ${buffer_size} &>> ${log}
  done
}

main "$@"
