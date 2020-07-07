#!/bin/bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This script is meant to be run as PID 1 inside a VM.
set -e

die() {
    echo "$1"
    exit 1
}

main() {
    local kind="$1"
    local src="$2"
    local mountpoint="$3"

    [[ "$$" == "1" ]] || die "Not runnnig as PID 1"

    [[ $# -eq 3 ]] || \
        die "Usage: $(basename "$0") <block|block_btrfs|direct|fs|fs_dax|p9> <src> <test directory>"

    [[ -d "${mountpoint}" ]] || [[ "${kind}" == "direct" ]]  || die "${mountpoint} is not a directory"

    # We are running as pid 1.  Mount some necessary file systems.
    mount -t proc proc /proc
    mount -t sysfs sys /sys
    mount -t tmpfs tmp /tmp
    mount -t tmpfs run /run

    case "${kind}" in
        block)
            [[ -b "${src}" ]] || die "${src} is not a block device"
            mkfs.ext4 "${src}"
            mount "${src}" "${mountpoint}"
            ;;
        block_btrfs)
            [[ -b "${src}" ]] || die "${src} is not a block device"
            mkfs.btrfs "${src}"
            mount "${src}" "${mountpoint}"
            ;;
        direct)
            mount "${src}" "/media/removable/USBDrive/"
            ;;
        p9)
            mount -t 9p \
                  -o "trans=virtio,version=9p2000.L,access=client,cache=loose" \
                  "${src}" "${mountpoint}"
            ;;
        fs)
            mount -t virtiofs "${src}" "${mountpoint}"
            ;;
        fs_dax)
            mount -t virtiofs -o dax "${src}" "${mountpoint}"
            ;;
        *)
            die "Unknown storage type: ${kind}"
    esac

    exec blogbench -d "${mountpoint}" -i 12

    case "${kind}" in
        direct)
            umount "/media/removable/USBDrive/"
            ;;
        *)
            sync
    esac
}

main "$@"
