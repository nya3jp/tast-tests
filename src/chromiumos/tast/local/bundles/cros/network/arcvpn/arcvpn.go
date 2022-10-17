// Copyright 2022 The ChromiumOS Authors
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

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/testing"
)

// These need to stay in sync with /vendor/google_arc/packages/system/ArcHostVpn
const (
	FacadeVPNPkg = "org.chromium.arc.hostvpn"
	FacadeVPNSvc = "org.chromium.arc.hostvpn.ArcHostVpnService"
)

// These need to stay in sync with
// //platform/tast-tests/android/ArcVpnTest/src/org/chromium/arc/testapp/arcvpn/ArcTestVpnService.java
const (
	VPNTestAppAPK = "ArcVpnTest.apk"
	VPNTestAppPkg = "org.chromium.arc.testapp.arcvpn"
	VPNTestAppAct = "org.chromium.arc.testapp.arcvpn.MainActivity"
	VPNTestAppSvc = "org.chromium.arc.testapp.arcvpn.ArcTestVpnService"
	TunIP         = "192.168.2.2"
)

// SetUpHostVPN creates a base VPN config, then calls SetUpHostVPNWithConfig
func SetUpHostVPN(ctx context.Context) (*vpn.Connection, action.Action, error) {
	// Host VPN config we'll use for connections. Arbitrary VPN type, but it can't cause the
	// test to log out of the user during setup otherwise we won't have access to adb anymore.
	// For example, vpn.AuthTypeCert VPNs will log the user out while trying to prep the cert
	// store.
	config := vpn.Config{
		Type:     vpn.TypeL2TPIPsec,
		AuthType: vpn.AuthTypePSK,
	}
	return SetUpHostVPNWithConfig(ctx, config)
}

// SetUpHostVPNWithConfig create the host VPN server, but does not initiate a connection. The
// returned vpn.Connection is immediately ready for Connect() to be called on it. Also returns a
// cleanup function that handles the VPN server cleanup for the caller to execute.
func SetUpHostVPNWithConfig(ctx context.Context, config vpn.Config) (*vpn.Connection, action.Action, error) {
	conn, err := vpn.NewConnection(ctx, config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create connection object")
	}
	if err := conn.SetUp(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to setup VPN")
	}
	return conn, func(ctx context.Context) error { return conn.Cleanup(ctx) }, nil
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

// WaitForARCServiceState checks if the Android service is running in the `expectedRunning` state.
func WaitForARCServiceState(ctx context.Context, a *arc.ARC, pkg, svc string, expectedRunning bool) error {
	testing.ContextLogf(ctx, "Check the state of %s/%s", pkg, svc)

	// Poll since it might take some time for the service to start/stop.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := a.Command(ctx, "dumpsys", "activity", "services", pkg+"/"+svc)
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
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "service not in expected running state of %t", expectedRunning)
	}
	return nil
}
