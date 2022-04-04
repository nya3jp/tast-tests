// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcvpn interacts with the ARC-side fake VPN.
package arcvpn

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/testing"
)

const (
	// These need to stay in sync with /vendor/google_arc/packages/system/ArcHostVpn

	// ARCVPNPackage is the package name of the ARC-side fake VPN
	ARCVPNPackage = "org.chromium.arc.hostvpn"

	// ARCVPNService is the name of the Android Service that runs the ARC-side fake VPN
	ARCVPNService = "ArcHostVpnService"
)

// SetUpHostVPN create the host VPN server, but does not initiate a connection. The returned
// vpn.Connection is immediately ready for Connect() to be called on it. Also returns a cleanup
// function that handles the VPN server cleanup for the caller to execute.
func SetUpHostVPN(ctx, cleanupCtx context.Context) (*vpn.Connection, func() error, error) {
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
		return nil, nil, errors.Wrap(err, "failed to create connection object")
	}
	if err := conn.SetUp(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to setup VPN")
	}
	return conn, func() error { return conn.Cleanup(cleanupCtx) }, nil
}

// SetARCVPNEnabled flips the flag in the current running ARC instance. If running multiple tests
// within the same ARC instance, it's recommended to cleanup by flipping the flag back to the
// expected default state afterwards. Since no state is persisted, new ARC instances will initialize
// with the default state.
func SetARCVPNEnabled(ctx context.Context, a *arc.ARC, enabled bool) error {
	testing.ContextLogf(ctx, "Setting cros-vpn-as-arc-vpn flag to %t", enabled)
	cmd := a.Command(ctx, "dumpsys", "wifi", "set-cros-vpn-as-arc-vpn", fmt.Sprintf("%t", enabled))
	o, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to execute 'set-cros-vpn-as-arc-vpn' commmand")
	}

	if !strings.Contains(string(o), "sEnableCrosVpnAsArcVpn="+fmt.Sprintf("%t", enabled)) {
		return errors.New("unable to set sEnableCrosVpnAsArcVpn to " + fmt.Sprintf("%t", enabled))
	}
	return nil
}

// CheckARCVPNState confirms if ArcHostVpnService is running in the 'expectedRunning' state.
func CheckARCVPNState(ctx context.Context, a *arc.ARC, expectedRunning bool) error {
	testing.ContextLog(ctx, "Check the state of ArcHostVpnService")

	// Poll since it might take some time for the service to start/stop
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := a.Command(ctx, "dumpsys", "activity", "services", ARCVPNPackage+"/."+ARCVPNService)
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

// ExpectARCPingSuccess checks if 'addr' is reachable over the 'network' in ARC. See
// ArcNetworkDebugTools#reachCmd for possible 'network' values.
func ExpectARCPingSuccess(ctx context.Context, a *arc.ARC, network, addr string) error {
	testing.ContextLogf(ctx, "Start to ping %s from ARC over %q", addr, network)
	// This polls for 5 seconds before it gives up on pinging from within ARC. We poll for a
	// little bit since the ARP table within ARC might not be populated yet - so give it some
	// time before the ping makes it through.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := a.Command(ctx, "dumpsys", "wifi", "tools", "reach", network, addr)
		o, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to execute 'reach' commmand")
		}

		if !strings.Contains(string(o), fmt.Sprintf("%s: reachable", addr)) {
			return errors.New("ping was unreachable")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "no response received in ARC")
	}

	return nil
}
