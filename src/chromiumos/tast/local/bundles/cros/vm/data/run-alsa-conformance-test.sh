#!/bin/bash
# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script is meant to be run as PID 1 inside a VM.
set -e

die() {
  echo "$1"
  exit 1
}

usage() {
  die "Usage: $(basename "$0")"
}

main() {
  [[ "$$" == "1" ]] || die "Not running as PID 1"

  [[ $# -ge 1 ]] || usage

  # We are running as pid 1.  Mount some necessary file systems.
  mount -t proc proc /proc
  mount -t sysfs sys /sys
  mount -t tmpfs tmp /tmp
  mount -t tmpfs run /run

  exec python alsa_conformance_test.py \
       --test-suites test_params test_rates test_all_pairs \
       -P hw:0,0
}

main "$@"
