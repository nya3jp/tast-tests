// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"strconv"
	"time"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWPacketCapture,
		Desc:     "Verifies `iw` interface connection behavior running a packet capture",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func IWPacketCapture(ctx context.Context, s *testing.State) {
	const (
		iface   = "mon0"
		wlan    = "wlan0"
		freq    = 2412 // This is Channel 1.
		fileOut = "/tmp/IWPacketCapture.pcap"
	)
	defer func() {
		// Cleanup routine.
		if err := iw.RemoveInterface(ctx, iface); err != nil {
			s.Error("RemoveInterface failed: ", err)
		}
		if err := testexec.CommandContext(ctx, "ip", "link", "set", wlan, "up").Run(testexec.DumpLogOnError); err != nil {
			s.Error("Could not bring back wlan0: ", err)
		}
	}()

	if err := iw.AddInterface(ctx, "phy0", iface, "monitor"); err != nil {
		s.Fatal("AddInterface failed: ", err)
	}
	if err := testexec.CommandContext(ctx, "ip", "link", "set", wlan, "down").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Could not tear down wlan0: ", err)
	}
	if err := testexec.CommandContext(ctx, "ip", "link", "set", iface, "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Could not bring up interface %s: %v", iface, err)
	}
	if err := testexec.CommandContext(ctx, "iw", "dev", iface, "set", "freq", strconv.Itoa(freq)).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Could not set %s frequency to %d: %v", iface, freq, err)
	}

	// Set packet capture timeout.
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel() // Cancel routine will execute before cleanup routine.
	if err := testexec.CommandContext(timeoutCtx, "/usr/libexec/debugd/helpers/capture_packets", iface, fileOut).Run(testexec.DumpLogOnError); err != nil {
		if timeoutCtx.Err() != context.DeadlineExceeded {
			s.Fatal("Packet capture binary failed: ", err)
		}
	}

	// Remove pcap.
	if err := os.Remove(fileOut); err != nil {
		s.Fatal("Could not removed pcap: ", err)
	}
}
