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

    net_dev=eth0
    ip a add "${addr}"/30 dev "${net_dev}"
    ip link set "${net_dev}" up
    ip route add default via "${gateway}"
    echo "nameserver 8.8.8.8" > /run/resolv.conf

    # Print IP info for debug
    ip a
    iptables -nL

    if [ ${role} == "server" ]; then
        exec iperf3 -V -s -p 1234 -1
    else
        # Ensure the server is ready.
        ping -c 5 "${dst_addr}"
        exec iperf3 -V -c "${dst_addr}" -p 1234 --connect-timeout 3000
    fi
}

main "$@"
