// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

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
		Func:     ARCVPNCrash,
		Desc:     "When ARC VPN crashes, host VPN is still reachable in ARC",
		Contacts: []string{"cassiewang@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "shillResetWithArcBooted",
		Params: []testing.Param{{
			Val:               "p",
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			Val:               "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ARCVPNCrash(ctx context.Context, s *testing.State) {
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

	// Check that if ArcHostVpnService is stopped unexpectedly (simulating some sort
	// of error), the host VPN is still reachable from within ARC.
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
		s.Fatalf("Failed to ping from ARC %s: %v", conn.Server.OverlayIP, err)
	}
	if err := crashARCVPN(ctx, a); err != nil {
		s.Fatal("Failed to crash ArcHostVpnService: ", err)
	}
	if err := arcvpn.CheckARCVPNState(ctx, a, false); err != nil {
		s.Fatal("ArcHostVpnService should be stopped, but isn't: ", err)
	}
	// VPN should still be reachable even without ArcHostVpnService
	if err := vpn.ExpectPingSuccess(ctx, pr, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn.Server.OverlayIP, err)
	}
	// arc0/eth0 are hardcoded in ARC as the iface of our fake ethernet network that VPN traffic
	// will fall back to if there's an issue with the ARC VPN.
	arc := s.Param().(string)
	network := "eth0"
	if arc == "p" {
		network = "arc0"
	}
	if err := arcvpn.ExpectARCPingSuccess(ctx, a, network, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s from ARC over %q: %v", conn.Server.OverlayIP, network, err)
	}
}

// crashARCVPN force-stops the ArcHostVpnService to simulate an unexpected stop (e.g. crash). This
// doesn't exercise normal ArcNetworkService->ArcHostVpnService service disconnection flows.
func crashARCVPN(ctx context.Context, a *arc.ARC) error {
	testing.ContextLog(ctx, "Stopping ArcHostVpnService")
	cmd := a.Command(ctx, "am", "force-stop", arcvpn.ARCVPNPackage)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to execute 'am force-stop' commmand")
	}

	if err := arcvpn.CheckARCVPNState(ctx, a, false); err != nil {
		return errors.Wrap(err, "ArcHostVpnService expected to be stopped, but was running")
	}
	return nil
}
