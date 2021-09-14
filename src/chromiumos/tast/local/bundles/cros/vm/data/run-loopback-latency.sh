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

  [[ $# -eq 1 ]] || \
      die "Usage: $(basename "$0") <log>"

  local log="$1"

  aplay -l
  arecord -l
  loopback_latency -i hw:0,0 -o hw:0,0 -n 1000 -l 5 -r 48000 -p 2048 -b 4096
}

main "$@"
