// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/arcvpn"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ARCToHostVPN,
		Desc:     "Switch from an ARC VPN to a host VPN",
		Contacts: []string{"cassiewang@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ARCToHostVPN(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a
	// few seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	a := s.FixtValue().(*arc.PreData).ARC

	if err := arcvpn.SetARCVPNEnabled(ctx, a, true); err != nil {
		s.Fatal("Failed to enable ARC VPN: ", err)
	}
	defer func() {
		if err := arcvpn.SetARCVPNEnabled(cleanupCtx, a, false); err != nil {
			s.Fatal("Failed to disable ARC VPN: ", err)
		}
	}()

	// Set up host VPN.
	conn, cleanup, err := arcvpn.SetUpHostVPN(ctx)
	if err != nil {
		s.Fatal("Failed to setup host VPN: ", err)
	}
	defer cleanup(cleanupCtx)

	// Install and start the test app.
	testing.ContextLog(ctx, "Installing ArcVpnTest.apk")
	if err := a.Install(ctx, arc.APKPath(arcvpn.VPNTestAppAPK)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	defer func() {
		testing.ContextLog(cleanupCtx, "Uninstalling ArcVpnTest.apk")
		if err := a.Uninstall(cleanupCtx, arcvpn.VPNTestAppPkg); err != nil {
			s.Fatal("Failed to uninstall ArcVpnTest.apk: ", err)
		}
	}()

	testing.ContextLog(ctx, "Preauthorizing ArcVpnTest")
	if _, err := a.Command(ctx, "dumpsys", "wifi", "authorize-vpn", arcvpn.VPNTestAppPkg).Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to execute authorize-vpn command: ", err)
	}

	testing.ContextLog(ctx, "Starting ArcVpnTest app")
	if _, err := a.Command(ctx, "am", "start", arcvpn.VPNTestAppPkg+"/"+arcvpn.VPNTestAppAct).Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start ArcVpnTest app activity: ", err)
	}

	// Make sure our test app is connected.
	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.VPNTestAppPkg, arcvpn.VPNTestAppSvc, true); err != nil {
		s.Fatalf("Failed to start %s: %v", arcvpn.VPNTestAppSvc, err)
	}

	// Host traffic gets routed to the ARC VPN correctly.
	if err := routing.ExpectPingSuccessWithTimeout(ctx, arcvpn.TunIP, "chronos", 10*time.Second); err != nil {
		s.Fatalf("Failed to ping %s from host: %v", arcvpn.TunIP, err)
	}

	// Connect to host VPN.
	if _, err := conn.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	}

	// Make sure our test app is disconnected.
	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.VPNTestAppPkg, arcvpn.VPNTestAppSvc, false); err != nil {
		s.Fatalf("Failed to start %s: %v", arcvpn.VPNTestAppSvc, err)
	}

	// Facade ARC vpn is connected.
	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.FacadeVPNPkg, arcvpn.FacadeVPNSvc, true); err != nil {
		s.Fatalf("Failed to start %s: %v", arcvpn.FacadeVPNSvc, err)
	}

	// Host and ARC traffic are routed correctly.
	if err := routing.ExpectPingSuccessWithTimeout(ctx, conn.Server.OverlayIP, "chronos", 10*time.Second); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn.Server.OverlayIP, err)
	}
	if err := arc.ExpectPingSuccess(ctx, a, "vpn", conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s from ARC over 'vpn': %v", conn.Server.OverlayIP, err)
	}

}
