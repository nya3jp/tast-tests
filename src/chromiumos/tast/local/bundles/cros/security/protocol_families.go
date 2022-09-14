// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"golang.org/x/sys/unix"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProtocolFamilies,
		Desc: "Compares available network protocol (address) families against a baseline",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func ProtocolFamilies(ctx context.Context, s *testing.State) {
	// Allowed protocol families. Note that these are confusingly also referred to as "address families"
	// in various places (including POSIX constant names), and that socket(2) calls them "communication domains".
	// See https://en.wikipedia.org/wiki/Berkeley_sockets#Protocol_and_address_families for more than you
	// probably want to know about this topic.
	allowedFamilies := make(map[int]struct{})
	for _, f := range []int{
		unix.AF_FILE,
		unix.AF_PACKET,
		unix.AF_INET,
		unix.AF_INET6,
		unix.AF_ROUTE,
		unix.AF_LOCAL,
		unix.AF_NETLINK,
		unix.AF_UNIX,
		unix.AF_BLUETOOTH,
		unix.AF_ALG,
		// May be present after vm tests load the vhost-vsock module
		unix.AF_VSOCK,
		// The underlying transport for configuration of Qualcomm integrated modems
		unix.AF_QIPCRTR,
	} {
		allowedFamilies[f] = struct{}{}
	}

	// Socket types to try to create.
	types := []int{
		unix.SOCK_STREAM,
		unix.SOCK_DGRAM,
		unix.SOCK_RAW,
		unix.SOCK_RDM,
		unix.SOCK_SEQPACKET,
	}

	// Tries to create network sockets of varying types for protocol family pf and returns true on success.
	// The socket type that succeeded is also returned.
	familyAvailable := func(pf int) (avail bool, typ int) {
		for _, typ := range types {
			fd, err := unix.Socket(pf, typ, 0)
			if err == nil {
				if err := unix.Close(fd); err != nil {
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
				// See AF_ and SOCK_ constants at https://godoc.org/golang.org/x/sys/unix#pkg-constants for names.
				s.Errorf("Protocol family %v unexpectedly allowed with type %v", pf, typ)
				if numFailed++; numFailed > maxFailed {
					s.Error("Too many errors; aborting")
					break
				}
			}
		}
	}
}
