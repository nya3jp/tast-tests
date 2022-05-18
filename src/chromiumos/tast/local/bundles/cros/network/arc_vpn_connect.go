// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/arcvpn"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ARCVPNConnect,
		Desc:     "Host VPN is mirrored with ARC VPN properly",
		Contacts: []string{"cassiewang@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "shillResetWithArcBooted",
		Params: []testing.Param{{
			// ExtraSoftwareDeps: []string{"android_p"},
			ExtraSoftwareDeps: []string{"android_p", "wireguard"},
		}, {
			Name: "vm",
			// ExtraSoftwareDeps: []string{"android_vm"},
			ExtraSoftwareDeps: []string{"android_vm", "wireguard"},
		}},
	})
}

// ARCVPNConnect differs from VPNConnect in that ARCVPNConnect focuses on
// testing the VPN that's started in ARC to mirror the host VPN. See b/147256449
// for more details. Much of the testing around host VPN setup is left to
// VPNConnect to verify.
func ARCVPNConnect(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a
	// few seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	// ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	ctx, _ = ctxutil.Shorten(cleanupCtx, 3*time.Second)
	// defer cancel()

	a := s.FixtValue().(*arc.PreData).ARC

	// conn, cleanup, err := arcvpn.SetUpHostVPN(ctx, cleanupCtx)
	conn, _, err := arcvpn.SetUpHostVPN(ctx, cleanupCtx)
	if err != nil {
		s.Fatal("Failed to setup host VPN: ", err)
	}
	// defer cleanup()

	// Verify ArcHostVpnService can connect and disconnect properly following the host VPN
	// lifecycle events.
	if err := arcvpn.SetARCVPNEnabled(ctx, a, true); err != nil {
		s.Fatal("Failed to enable ARC VPN: ", err)
	}
	// defer func() {
	// 	if err := arcvpn.SetARCVPNEnabled(ctx, a, false); err != nil {
	// 		s.Fatal("Failed to disable ARC VPN: ", err)
	// 	}
	// }()
	if _, err := conn.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	}
	if err := arcvpn.CheckARCVPNState(ctx, a, true); err != nil {
		s.Fatal("Failed to start ArcHostVpnService: ", err)
	}

	// wg0Pcap, _ := testing.ContextOutDir(ctx)
	// wg0Pcap += ".wg0.pcap"
	// testing.ContextLogf(ctx, "wg0Pcap file: ", wg0Pcap)

	// g, ctx := errgroup.WithContext(ctx)
	// g.Go(func() error {
	// 	tcpdump := testexec.CommandContext(ctx, "/usr/local/sbin/tcpdump", "-ni", "wg0", "--immediate-mode", "-w", wg0Pcap)
	// 	if err := tcpdump.Run(testexec.DumpLogOnError); err != nil {
	// 		s.Fatal("Unable to execute tcpdump:", err)

	// 	}
	// 	return nil
	// })

	ppp0Pcap, _ := testing.ContextOutDir(ctx)
	ppp0Pcap += ".ppp0.pcap"
	testing.ContextLog(ctx, "ppp0Pcap file: ", ppp0Pcap)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		tcpdump := testexec.CommandContext(ctx, "/usr/local/sbin/tcpdump", "-ni", "ppp0", "--immediate-mode", "-w", ppp0Pcap)
		if err := tcpdump.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Unable to execute tcpdump: ", err)

		}
		return nil
	})

	arcbr0Pcap, _ := testing.ContextOutDir(ctx)
	arcbr0Pcap += ".l2tp_arcbr0.pcap"
	testing.ContextLog(ctx, "l2tp_arcbr0Pcap file: ", arcbr0Pcap)

	g, ctx = errgroup.WithContext(ctx)
	g.Go(func() error {
		tcpdump := testexec.CommandContext(ctx, "/usr/local/sbin/tcpdump", "-ni", "arcbr0", "--immediate-mode", "-w", arcbr0Pcap)
		if err := tcpdump.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Unable to execute tcpdump: ", err)

		}
		return nil
	})

	pr := localping.NewLocalRunner()
	if err := vpn.ExpectPingSuccess(ctx, pr, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn.Server.OverlayIP, err)
	}

	// s.Fatal("cassiewang failing on purpose here")

	if err := arcvpn.ExpectARCPingSuccess(ctx, a, "vpn", conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s from ARC over 'vpn': %v", conn.Server.OverlayIP, err)
	}

	// Disconnect
	if err := conn.Disconnect(ctx); err != nil {
		s.Error("Failed to disconnect VPN: ", err)
	}
	if err := arcvpn.CheckARCVPNState(ctx, a, false); err != nil {
		s.Fatal("ArcHostVpnService should be stopped, but isn't: ", err)
	}
	if err := arcvpn.ExpectARCPingSuccess(ctx, a, "vpn", conn.Server.OverlayIP); err == nil {
		s.Fatalf("Expected unable to ping %s from ARC over 'vpn', but was reachable", conn.Server.OverlayIP)
	}

	// Reconnect
	if err := waitForConnect(ctx, conn); err != nil {
		s.Fatal("Failed to reconnect to VPN: ", err)
	}
	if err := arcvpn.CheckARCVPNState(ctx, a, true); err != nil {
		s.Fatal("Failed to start ArcHostVpnService on reconnection: ", err)
	}
	if err := vpn.ExpectPingSuccess(ctx, pr, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn.Server.OverlayIP, err)
	}
	if err := arcvpn.ExpectARCPingSuccess(ctx, a, "vpn", conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s from ARC over 'vpn': %v", conn.Server.OverlayIP, err)
	}
}

func waitForConnect(ctx context.Context, conn *vpn.Connection) error {
	// Reconnecting right after a disconnect takes some time for the reconnection to succeed.
	// Poll for a bit since it should be a transient issue.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := conn.Connect(ctx)
		if err != nil {
			return err
		}
		if !connected {
			return errors.New("unable to connect to VPN")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}
