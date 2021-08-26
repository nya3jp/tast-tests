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
  local playback_log="$1"
  local capture_log="$2"

  [[ "$$" == "1" ]] || die "Not runnnig as PID 1"

  [[ $# -eq 2 ]] || \
      die "Usage: $(basename "$0") <playback_log> <capture_log>"

  # We are running as pid 1.  Mount some necessary file systems.
  mount -t tmpfs tmp /tmp
  mount -t tmpfs run /run

  export PYTHONHOME=/usr/local
  export PATH=${PATH}:/usr/local/bin
  # TODO: Remove /usr/bin before submitting. After crrev.com/c/3138160 is submitted <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<,
  /usr/bin/alsa_conformance_test.py \
      --test-suites test_params test_rates test_all_pairs \
      -P hw:0,0 --rate-criteria-diff-pct 0.1 \
      --json-file "${playback_log}"
  /usr/bin/alsa_conformance_test.py \
      --test-suites test_params test_rates test_all_pairs \
      -C hw:0,0 --rate-criteria-diff-pct 0.1 \
      --json-file "${capture_log}"
}

main "$@"
