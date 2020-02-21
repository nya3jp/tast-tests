#!/bin/bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script is meant to be run as PID 1 inside a VM.
set -e

shopt -s globstar

die() {
    echo "$1"
    exit 1
}

main() {
    local pjdfstest=/usr/local/opt/pjdfstest
    local testdir="$1"
    local mountdir="${testdir}/pjdfstest"

    [[ "$$" == "1" ]] || die "Not runnnig as PID 1"

    [[ -n "${testdir}" ]] || die "Usage: $(basename "$0") <test directory>"

    [[ -d "${testdir}" ]] || die "${testdir} is not a directory"

    [[ -d "${pjdfstest}" ]] || die "${pjdfstest} doesn't exist"

    # We are running as pid 1.  Mount some necessary file systems.
    mount -t proc proc /proc
    mount -t sysfs sys /sys
    mount -t tmpfs tmp /tmp
    mount -t tmpfs run /run

    mkdir -p "${mountdir}"
    mount --bind "${pjdfstest}" "${mountdir}"

    cd "${testdir}"
    exec runtests -v ./pjdfstest/tests/**/*.t
}

main "$@"
