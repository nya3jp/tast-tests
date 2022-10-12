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

const (
	apk = "ArcVpnTest.apk"
	pkg = "org.chromium.arc.testapp.arcvpn"
	act = "org.chromium.arc.testapp.arcvpn.MainActivity"
	svc = "org.chromium.arc.testapp.arcvpn.ArcTestVpnService"

	// This value comes from the address being set in
	// //platform/tast-tests/android/ArcVpnTest/src/org/chromium/arc/testapp/arcvpn/ArcTestVpnService.java
	tunIP = "192.168.2.2"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HostToARCVPN,
		Desc:     "Switch from a host VPN to an ARC VPN",
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

func HostToARCVPN(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a
	// few seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	a := s.FixtValue().(*arc.PreData).ARC

	// Set up and connect to host VPN.
	conn, cleanup, err := arcvpn.SetUpHostVPN(ctx)
	if err != nil {
		s.Fatal("Failed to setup host VPN: ", err)
	}
	defer cleanup(cleanupCtx)

	if err := arcvpn.SetARCVPNEnabled(ctx, a, true); err != nil {
		s.Fatal("Failed to enable ARC VPN: ", err)
	}
	defer func() {
		if err := arcvpn.SetARCVPNEnabled(cleanupCtx, a, false); err != nil {
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
		s.Fatalf("Failed to ping %s from ARC over 'vpn': %v", conn.Server.OverlayIPv4, err)
	}

	// Install and start the test app.
	testing.ContextLog(ctx, "Installing ArcVpnTest.apk")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	defer func() {
		testing.ContextLog(cleanupCtx, "Uninstalling ArcVpnTest.apk")
		if err := a.Uninstall(cleanupCtx, pkg); err != nil {
			s.Fatal("Failed to uninstall ArcVpnTest.apk: ", err)
		}
	}()

	testing.ContextLog(ctx, "Preauthorizing ArcVpnTest")
	if _, err := a.Command(ctx, "dumpsys", "wifi", "authorize-vpn", pkg).Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to execute authorize-vpn command: ", err)
	}

	testing.ContextLog(ctx, "Starting ArcVpnTest app")
	if _, err := a.Command(ctx, "am", "start", pkg+"/"+act).Output(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start ArcVpnTest app activity: ", err)
	}

	// Make sure the host is disconnected.
	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.Pkg, arcvpn.Svc, false); err != nil {
		s.Fatalf("Failed to stop %s: %v", arcvpn.Svc, err)
	}

	// Make sure our test app is connected.
	if err := arcvpn.WaitForARCServiceState(ctx, a, pkg, svc, true); err != nil {
		s.Fatalf("Failed to start %s: %v", svc, err)
	}

	// Host traffic gets routed to the ARC VPN correctly.
	if err := routing.ExpectPingSuccessWithTimeout(ctx, tunIP, "chronos", 10*time.Second); err != nil {
		s.Fatalf("Failed to ping %s from host: %v", tunIP, err)
	}
}
