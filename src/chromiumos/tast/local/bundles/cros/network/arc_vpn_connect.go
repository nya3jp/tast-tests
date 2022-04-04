// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
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

	// Host VPN config we'll use for connections. Arbitrary VPN type, but it can't cause the
	// test to log out of the user during setup otherwise we won't have access to adb anymore.
	// For example, vpn.AuthTypeCert VPNs will log the user out while trying to prep the cert
	// store.
	config := vpn.Config{
		Type:     vpn.TypeL2TPIPsecSwanctl,
		AuthType: vpn.AuthTypePSK,
	}
	conn, err := vpn.NewConnection(ctx, config)
	if err != nil {
		s.Fatal("Failed to create connection object: ", err)
	}
	if err := conn.SetUp(ctx); err != nil {
		s.Fatal("Failed to setup VPN: ", err)
	}
	defer func() {
		if err := conn.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up connection: ", err)
		}
	}()

	// Case 1: Check that ArcHostVpnService doesn't start if the flag isn't enabled. And the
	// host VPN is still reachable from within ARC. Purposely don't defer VPN cleanup. We'll
	// explicitly cleanup later as part of the test
	if err := waitForConnect(ctx, conn); err != nil {
		s.Fatal("Failed to connect to VPN: ", err)
	}
	if err := checkARCVPNState(ctx, a, false); err != nil {
		s.Fatal("ArcHostVpnService not supposed to be running: ", err)
	}
	if err := conn.Disconnect(ctx); err != nil {
		s.Error("Failed to disconnect VPN: ", err)
	}

	// Case 2: With the proper flag enabled, verify ArcHostVpnService connects, can reach the
	// host VPN, and disconnects normally following the host VPN lifecycle events.
	if err := enableARCVPN(ctx, a); err != nil {
		s.Fatal("Failed to enable ARC VPN: ", err)
	}
	if err := waitForConnect(ctx, conn); err != nil {
		s.Fatal("Failed to connect to VPN: ", err)
	}
	if err := checkARCVPNState(ctx, a, true); err != nil {
		s.Fatal("Failed to start ArcHostVpnService: ", err)
	}
	pr := localping.NewLocalRunner()
	if err := expectPingSuccesses(ctx, pr, a, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s: %v", conn.Server.OverlayIP, err)
	}
	if err := conn.Disconnect(ctx); err != nil {
		s.Error("Failed to disconnect VPN: ", err)
	}
	if err := checkARCVPNState(ctx, a, false); err != nil {
		s.Fatal("Failed to stop ArcHostVpnService: ", err)
	}

	// Case 3: Check that if ArcHostVpnService is stopped unexpectedly (simulating some sort
	// of error), the host VPN is still reachable from within ARC.
	if err := waitForConnect(ctx, conn); err != nil {
		s.Fatal("Failed to connect to VPN: ", err)
	}
	if err := checkARCVPNState(ctx, a, true); err != nil {
		s.Fatal("Failed to start ArcHostVpnService: ", err)
	}
	if err := expectPingSuccesses(ctx, pr, a, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s: %v", conn.Server.OverlayIP, err)
	}
	if err := stopARCVPN(ctx, a); err != nil {
		s.Fatal("Failed to force-stop ArcHostVpnService: ", err)
	}
	// VPN should still be reachable even without ArcHostVpnService
	if err := expectPingSuccesses(ctx, pr, a, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s: %v", conn.Server.OverlayIP, err)
	}
}

func waitForConnect(ctx context.Context, conn *vpn.Connection) error {
	// Disconnecting then reconnecting takes some time for the reconnection to succeed. Poll
	// for a bit since it should be a transient issue.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := conn.Connect(ctx)
		if err != nil {
			return err
		}
		if !connected {
			return errors.New("unable to connect to VPN")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	return nil
}

func enableARCVPN(ctx context.Context, a *arc.ARC) error {
	testing.ContextLog(ctx, "Enabling cros-vpn-as-arc-vpn flag")
	cmd := a.Command(ctx, "dumpsys", "wifi", "set-cros-vpn-as-arc-vpn", "true")
	o, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to execute 'set-cros-vpn-as-arc-vpn' commmand")
	}

	if !strings.Contains(string(o), "sEnableCrosVpnAsArcVpn=true") {
		return errors.New("unable to enable sEnableCrosVpnAsArcVpn")
	}
	return nil
}

// stopARCVPN force-stops the ArcHostVpnService to simulate an unexpected stop (e.g. crash). This
// doesn't exercise normal ArcNetworkService->ArcHostVpnService service disconnection flows.
func stopARCVPN(ctx context.Context, a *arc.ARC) error {
	testing.ContextLog(ctx, "Stopping ArcHostVpnService")
	cmd := a.Command(ctx, "am", "force-stop", "org.chromium.arc.hostvpn")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to execute 'am force-stop' commmand")
	}

	if err := checkARCVPNState(ctx, a, false); err != nil {
		return errors.Wrap(err, "ArcHostVpnService expected to be stopped, but was running")
	}
	return nil
}

func checkARCVPNState(ctx context.Context, a *arc.ARC, expectedRunning bool) error {
	testing.ContextLog(ctx, "Check the state of ArcHostVpnService")

	// Poll since it might take some time for the service to start/stop
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := a.Command(ctx, "dumpsys", "activity", "services", "org.chromium.arc.hostvpn/.ArcHostVpnService")
		o, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to execute 'dumpsys activity services' commmand")
		}

		// Use raw string so we can directly use backslashes
		matched, matchErr := regexp.Match(`ServiceRecord\{`, o)
		if matched != expectedRunning || matchErr != nil {
			if expectedRunning {
				return errors.Wrap(matchErr, "expected, but didn't find ServiceRecord")
			}
			return errors.Wrap(matchErr, "didn't expect, but found ServiceRecord")
		}

		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrapf(err, "service not in expected running state of %t", expectedRunning)
	}
	return nil
}

func expectPingSuccesses(ctx context.Context, pr *ping.Runner, a *arc.ARC, addr string) error {
	testing.ContextLogf(ctx, "Start to ping %s", addr)

	// Ping from the host
	res, err := pr.Ping(ctx, addr, ping.Count(3), ping.User("chronos"))
	if err != nil {
		return err
	}
	if res.Received == 0 {
		return errors.New("no response received on host")
	}

	// This polls for 5 seconds before it gives up on pinging from within ARC. We poll for a
	// little bit since the ARP table within ARC might not be populated yet - so give it some
	// time before the ping makes it through.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := a.Command(ctx, "dumpsys", "wifi", "tools", "reach", addr)
		o, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to execute 'reach' commmand")
		}

		if !strings.Contains(string(o), fmt.Sprintf("%s: reachable", addr)) {
			return errors.New("ping was unreachable")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "no response received in ARC")
	}

	return nil
}
