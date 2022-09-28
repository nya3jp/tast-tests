// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bufio"
	"context"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SupportedProtocols,
		Desc: "Checks that required network protocols are supported by the kernel",
		Contacts: []string{
			"cros-networking@google.com",
			"hugobenichi@google.com",
			"chromeos-kernel-test@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func SupportedProtocols(ctx context.Context, s *testing.State) {
	required := make(map[string]struct{}) // protocols required to be in /proc/net/protocols

	// Create sockets to try to ensure that the proper kernel modules are
	// loaded in order for the required protocols to be listed in /proc.
	for _, info := range []struct {
		name                  string // value from "protocol" field in /proc/net/protocols
		domain, typ, protocol int    // socket info to load kernel module
	}{
		{"RFCOMM", unix.AF_BLUETOOTH, unix.SOCK_STREAM, unix.BTPROTO_RFCOMM},
		{"RFCOMM", unix.AF_BLUETOOTH, unix.SOCK_SEQPACKET, unix.BTPROTO_SCO},
		{"L2CAP", unix.AF_BLUETOOTH, unix.SOCK_STREAM, unix.BTPROTO_L2CAP},
		{"HCI", unix.AF_BLUETOOTH, unix.SOCK_RAW, unix.BTPROTO_HCI},
		{"PACKET", unix.AF_PACKET, unix.SOCK_DGRAM, 0},
		{"RAWv6", unix.AF_INET6, unix.SOCK_RAW, 0},
		{"UDPLITEv6", unix.AF_INET6, unix.SOCK_DGRAM, unix.IPPROTO_UDPLITE},
		{"UDPv6", unix.AF_INET6, unix.SOCK_DGRAM, 0},
		{"TCPv6", unix.AF_INET6, unix.SOCK_STREAM, 0},
		{"UNIX", unix.AF_UNIX, unix.SOCK_STREAM, 0},
		{"UDP-Lite", unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDPLITE},
		{"PING", unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_ICMP},
		{"RAW", unix.AF_INET, unix.SOCK_RAW, 0},
		{"UDP", unix.AF_INET, unix.SOCK_DGRAM, 0},
		{"TCP", unix.AF_INET, unix.SOCK_STREAM, 0},
		{"NETLINK", unix.AF_NETLINK, unix.SOCK_DGRAM, 0},
	} {
		required[info.name] = struct{}{}

		// Log but discard errors; we're just doing this to load modules.
		if fd, err := unix.Socket(info.domain, info.typ, info.protocol); err != nil {
			s.Logf("Couldn't create socket (%v, %v, %v) for %v: %v",
				info.domain, info.typ, info.protocol, info.name, err)
		} else if err := unix.Close(fd); err != nil {
			s.Errorf("Failed to close socket (%v, %v, %v) for %v: %v",
				info.domain, info.typ, info.protocol, info.name, err)
		}
	}

	// Now read the kernel's protocol stats and verify that all of the expected
	// protocols are listed.
	const path = "/proc/net/protocols"
	f, err := os.Open(path)
	if err != nil {
		s.Fatal("Failed to open protocols: ", err)
	}
	defer f.Close()

	loaded := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) < 1 || parts[0] == "protocol" { // skip last line and header
			continue
		}
		loaded[parts[0]] = struct{}{}
	}
	if sc.Err() != nil {
		s.Fatal("Failed to read protocols: ", err)
	}

	for name := range required {
		if _, ok := loaded[name]; !ok {
			s.Errorf("%v protocol missing in %v", name, path)
		}
	}
}
