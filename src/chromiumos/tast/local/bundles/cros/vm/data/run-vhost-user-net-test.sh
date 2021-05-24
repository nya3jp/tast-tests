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

main() {
    local role="$1"
    local addr="$2"
    local gateway="$3"
    local dst_addr="$4" # only for client

    # We are running as pid 1.  Mount some necessary file systems.
    mount -t proc proc /proc
    mount -t sysfs sys /sys
    mount -t tmpfs tmp /tmp
    mount -t tmpfs run /run

    GUEST_DEV=eth0
    ip a add "${addr}"/30 dev "${GUEST_DEV}"
    ip link set "${GUEST_DEV}" up
    ip route add default via "${gateway}"
    echo "nameserver 8.8.8.8" > /run/resolv.conf

    if [ ${role} == "server" ]; then
        ip a
        iptables -nL
        iperf3 -s -p 1234 -1
    else
        ip a
        iptables -nL
        iperf3 -c "${dst_addr}" -p 1234
    fi
}

main "$@"
