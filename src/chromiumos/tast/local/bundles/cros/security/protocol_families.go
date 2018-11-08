// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"syscall"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProtocolFamilies,
		Desc: "Compares available network protocol (address) families against a baseline",
		Attr: []string{"informational"},
	})
}

func ProtocolFamilies(ctx context.Context, s *testing.State) {
	// Allowed protocol families. Note that these are confusingly also referred to as "address families"
	// in various places (including POSIX constant names), and that socket(2) calls them "communication domains".
	// See https://en.wikipedia.org/wiki/Berkeley_sockets#Protocol_and_address_families for more than you
	// probably want to know about this topic.
	allowedFamilies := make(map[int]struct{})
	for _, f := range []int{
		syscall.AF_FILE,
		syscall.AF_PACKET,
		syscall.AF_INET,
		syscall.AF_INET6,
		syscall.AF_ROUTE,
		syscall.AF_LOCAL,
		syscall.AF_NETLINK,
		syscall.AF_UNIX,
		syscall.AF_BLUETOOTH,
		syscall.AF_ALG,
		// TODO(derat): The security_ProtocolFamilies Autotest test also permits PF_QIPCRTR (42) for the cheza
		// board. No cheza devices exist in the lab, but consider adding 42 to this list in the future if needed.
	} {
		allowedFamilies[f] = struct{}{}
	}

	// Socket types to try to create.
	types := []int{
		syscall.SOCK_STREAM,
		syscall.SOCK_DGRAM,
		syscall.SOCK_RAW,
		syscall.SOCK_RDM,
		syscall.SOCK_SEQPACKET,
	}

	// Tries to create network sockets of varying types for protocol family pf and returns true on success.
	// The socket type that succeeded is also returned.
	familyAvailable := func(pf int) (avail bool, typ int) {
		for _, typ := range types {
			fd, err := syscall.Socket(pf, typ, 0)
			if err == nil {
				if err := syscall.Close(fd); err != nil {
					s.Errorf("Failed to close socket with protocol family %v and type %v: %v", pf, typ, err)
				}
				return true, typ
			}
		}
		return false, 0
	}

	numFailed := 0
	const maxFailed = 10

	// Iterate over a larger range of families than what's currently defined in order to catch future additions.
	for pf := 0; pf < 256; pf++ {
		if avail, typ := familyAvailable(pf); avail {
			if _, allowed := allowedFamilies[pf]; allowed {
				s.Logf("Protocol family %v allowed with type %v", pf, typ)
			} else {
				// See AF_ and SOCK_ constants at https://golang.org/pkg/syscall/#pkg-constants for names.
				s.Errorf("Protocol family %v unexpectedly allowed with type %v", pf, typ)
				if numFailed++; numFailed > maxFailed {
					s.Error("Too many errors; aborting")
					break
				}
			}
		}
	}
}
