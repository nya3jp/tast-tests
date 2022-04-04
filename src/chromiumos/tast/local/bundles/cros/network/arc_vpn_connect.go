// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

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
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
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
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	defer cancel()

	a := s.FixtValue().(*arc.PreData).ARC

	conn, cleanup, err := arcvpn.SetUpHostVPN(ctx, cleanupCtx)
	if err != nil {
		s.Fatal("Failed to setup host VPN: ", err)
	}
	defer cleanup()

	// Verify ArcHostVpnService can connect and disconnect properly following the host VPN
	// lifecycle events.
	if err := arcvpn.SetARCVPNEnabled(ctx, a, true); err != nil {
		s.Fatal("Failed to enable ARC VPN: ", err)
	}
	defer func() {
		if err := arcvpn.SetARCVPNEnabled(ctx, a, false); err != nil {
			s.Fatal("Failed to disable ARC VPN: ", err)
		}
	}()
	if _, err := conn.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	}
	if err := arcvpn.CheckARCVPNState(ctx, a, true); err != nil {
		s.Fatal("Failed to start ArcHostVpnService: ", err)
	}
	pr := localping.NewLocalRunner()
	if err := vpn.ExpectPingSuccess(ctx, pr, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn.Server.OverlayIP, err)
	}
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
