// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/network/routing"
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

	conn, cleanup, err := arcvpn.SetUpHostVPN(ctx)
	if err != nil {
		s.Fatal("Failed to setup host VPN: ", err)
	}
	defer cleanup(cleanupCtx)

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
	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.Pkg, arcvpn.Svc, true); err != nil {
		s.Fatalf("Failed to start %s: %v", arcvpn.Svc, err)
	}
	if err := routing.ExpectPingSuccessWithTimeout(ctx, conn.Server.OverlayIPv4, "chronos", 10*time.Second); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn.Server.OverlayIPv4, err)
	}
	if err := arc.ExpectPingSuccess(ctx, a, "vpn", conn.Server.OverlayIPv4); err != nil {
		s.Fatalf("Failed to ping from ARC %s: %v", conn.Server.OverlayIPv4, err)
	}
	if err := crashARCVPN(ctx, a); err != nil {
		s.Fatal("Failed to crash ArcHostVpnService: ", err)
	}
	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.Pkg, arcvpn.Svc, false); err != nil {
		s.Fatalf("Failed to stop %s: %v", arcvpn.Svc, err)
	}
	// VPN should still be reachable even without ArcHostVpnService
	if err := routing.ExpectPingSuccessWithTimeout(ctx, conn.Server.OverlayIPv4, "chronos", 10*time.Second); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn.Server.OverlayIPv4, err)
	}
	// arc0/eth0 are hardcoded in ARC as the iface of our fake ethernet network that VPN traffic
	// will fall back to if there's an issue with the ARC VPN.
	arcVersion := s.Param().(string)
	network := "eth0"
	if arcVersion == "p" {
		network = "arc0"
	}
	if err := arc.ExpectPingSuccess(ctx, a, network, conn.Server.OverlayIPv4); err != nil {
		s.Fatalf("Failed to ping %s from ARC over %q: %v", conn.Server.OverlayIPv4, network, err)
	}
}

// crashARCVPN force-stops the ArcHostVpnService to simulate an unexpected stop (e.g. crash). This
// doesn't exercise normal ArcNetworkService->ArcHostVpnService service disconnection flows.
func crashARCVPN(ctx context.Context, a *arc.ARC) error {
	testing.ContextLog(ctx, "Stopping ArcHostVpnService")
	cmd := a.Command(ctx, "am", "force-stop", arcvpn.Pkg)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to execute 'am force-stop' commmand")
	}

	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.Pkg, arcvpn.Svc, false); err != nil {
		return errors.Wrapf(err, "failed to stop %s", arcvpn.Svc)
	}
	return nil
}
