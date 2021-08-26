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

  [[ $# -eq 2 ]] || \
      die "Usage: $(basename "$0") <playback_log> <capture_log>"

  local playback_log="$1"
  local capture_log="$2"

  # All the sample rates supported by virtio-snd
  local rates=(5512 8000 11025 16000 22050 32000 44100 48000 64000 88200 96000
      176400 192000 384000)

  export PYTHONHOME=/usr/local
  export PATH=${PATH}:/usr/local/bin
  /usr/bin/alsa_conformance_test.py \
      --test-suites test_params test_rates test_all_pairs \
      -P hw:0,0 --rate-criteria-diff-pct 0.1 \
      --json-file "${playback_log}" \
      --allow-rates "${rates[@]}"
  /usr/bin/alsa_conformance_test.py \
      --test-suites test_params test_rates test_all_pairs \
      -C hw:0,0 --rate-criteria-diff-pct 0.1 \
      --json-file "${capture_log}" \
      --allow-rates "${rates[@]}"
}

main "$@"
