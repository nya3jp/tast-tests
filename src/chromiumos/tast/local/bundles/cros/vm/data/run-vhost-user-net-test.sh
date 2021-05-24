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

main() {
    # We are running as pid 1.  Mount some necessary file systems.
    mount -t proc proc /proc
    mount -t sysfs sys /sys
    mount -t tmpfs tmp /tmp
    mount -t tmpfs run /run

    GUEST_DEV=eth0
    ip a add 10.0.2.1/24 dev $GUEST_DEV
    ip link set $GUEST_DEV up
    ip route add default via 10.0.2.2
    echo "nameserver 8.8.8.8" > /run/resolv.conf
    ping -c 1 8.8.8.8
}

main "$@"
