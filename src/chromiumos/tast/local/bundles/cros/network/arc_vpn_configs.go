// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/arcvpn"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCVPNConfigs,
		Desc:         "Host VPN configs are reflected properly in ARC VPN",
		Contacts:     []string{"cassiewang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "shillResetWithArcBooted",
		SoftwareDeps: []string{"arc", "wireguard"},
	})
}

// ARCVPNConfigs tests that a few specific config fields from the host VPN are passed and set on
// the mirrored ARC VPN correctly.
func ARCVPNConfigs(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC

	if err := arcvpn.SetARCVPNEnabled(ctx, a, true); err != nil {
		s.Fatal("Failed to enable ARC VPN: ", err)
	}
	defer func() {
		if err := arcvpn.SetARCVPNEnabled(ctx, a, false); err != nil {
			s.Fatal("Failed to disable ARC VPN: ", err)
		}
	}()

	// Connect with our first config and verify values.
	//
	// We specifically don't use a L2TP type because shill overrides the MTU value into a
	// hardcoded value. This eventually gets set properly again on the host-side, but Chrome
	// passes the overridden value to ARC so it won't get reflected properly in ARC. Note that
	// the hardcoded value is a minimum valid MTU size so it doesn't break any correctness.
	if err := verifyVPNWithConfig(ctx, a, vpn.Config{
		Type:          vpn.TypeWireGuard,
		Metered:       false,
		SearchDomains: []string{"foo1", "bar1"},
		MTU:           576,
	}); err != nil {
		s.Fatal("Failed to verify VPN connection with the first config: ", err)
	}

	// Connect with a different config and verify values. Use values that are different from
	// the first connection's config's values to ensure we didn't just get lucky with some
	// default values.
	if err := verifyVPNWithConfig(ctx, a, vpn.Config{
		Type:          vpn.TypeWireGuard,
		Metered:       true,
		SearchDomains: []string{"foo2", "bar2"},
		MTU:           1280,
	}); err != nil {
		s.Fatal("Failed to verify VPN connection with the second config: ", err)
	}
}

func verifyVPNWithConfig(ctx context.Context, a *arc.ARC, config vpn.Config) error {
	// If the main body of the function times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 6*time.Second)
	defer cancel()

	conn, cleanup, err := arcvpn.SetUpHostVPNWithConfig(ctx, config)
	if err != nil {
		return errors.Wrap(err, "failed to setup host VPN")
	}
	defer cleanup(cleanupCtx)
	if _, err := conn.Connect(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to VPN server")
	}
	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.Pkg, arcvpn.Svc, true); err != nil {
		return errors.Wrapf(err, "failed to start %s", arcvpn.Svc)
	}
	if err := routing.ExpectPingSuccessWithTimeout(ctx, conn.Server.OverlayIPv4, "chronos", 10*time.Second); err != nil {
		return errors.Wrapf(err, "failed to ping from host %s", conn.Server.OverlayIPv4)
	}
	if err := arc.ExpectPingSuccess(ctx, a, "vpn", conn.Server.OverlayIPv4); err != nil {
		return errors.Wrapf(err, "failed to ping %s from ARC over 'vpn'", conn.Server.OverlayIPv4)
	}
	cmd := a.Command(ctx, "dumpsys", "wifi", "networks", "transport", "vpn")
	o, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to execute 'dumpsys wifi networks transport vpn'")
	}
	oStr := string(o)
	// On P, the VpnService.Builder#setMetered API isn't available for us to override the value
	// and VPNs are considered metered by default.
	arcVersion, err := arc.SDKVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get ARC SDK version")
	}
	if arcVersion == arc.SDKP {
		config.Metered = true
	}
	if err := checkMatch(oStr, `capabilities=.*`, `NOT_METERED`, !config.Metered); err != nil {
		return errors.Wrap(err, "failed to verify capabilities on ARC VPN network")
	}
	for _, domain := range config.SearchDomains {
		if err := checkMatch(oStr, `domains=.*`, domain, true); err != nil {
			return errors.Wrap(err, "failed to verify search domains on ARC VPN network")
		}
	}
	// Use the output of ifconfig instead of dumpsys because Android P doesn't set the MTU
	// property on the VPN's LinkProperties (which is what the dumpsys reads from). So the
	// dumpsys output will always report a MTU of 0 on P (this is fixed in R). ifconfig reports
	// it correctly on both P and R.
	// TODO: We can switch to the dumpsys output once b/233322908 is fixed.
	cmd = a.Command(ctx, "ifconfig", "tun0")
	o, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to execute 'ifconfig tun0'")
	}
	if err := checkMatch(string(o), `MTU:.*`, fmt.Sprint(config.MTU), true); err != nil {
		return errors.Wrap(err, "failed to verify MTU on ARC VPN network")
	}

	// Disconnect from the connection. Verify the state and connectivity in ARC.
	if err := conn.Disconnect(ctx); err != nil {
		return errors.Wrap(err, "failed to disconnect VPN")
	}
	if err := arcvpn.WaitForARCServiceState(ctx, a, arcvpn.Pkg, arcvpn.Svc, false); err != nil {
		return errors.Wrapf(err, "failed to stop %s", arcvpn.Svc)
	}
	if err := arc.ExpectPingSuccess(ctx, a, "vpn", conn.Server.OverlayIPv4); err == nil {
		return errors.Errorf("expected unable to ping %s from ARC over 'vpn', but was reachable", conn.Server.OverlayIPv4)
	}

	return nil
}

// checkMatch will check that there is some 'lineRegex' within 'input'. And within the matches
// that matched the 'lineRegex', we should/shouldn't 'match' a 'valueRegex'.
//
// For example, given:
//
//	input:
//	        "foo=hello\n
//	        bar=world jupiter"
//	lineRegex:
//	        `bar=.*`
//	valueRegex:
//	        `world`
//	match:
//	        true
//
// In the example above, the 'lineRegex' would match 'bar=world jupiter'. And we expect
// to find 'world' somewhere within that line. The 'lineRegex' must be found within the given
// 'input'. Whether the 'valueRegex' is expected in the submatch is up to 'match'.
func checkMatch(input, lineRegex, valueRegex string, expectMatch bool) error {
	// Narrow down the input to the target lineRegex
	lineRe := regexp.MustCompile(lineRegex)
	lineMatches := lineRe.FindAllString(input, -1)
	if len(lineMatches) == 0 {
		return errors.Errorf("failed to find any lines that matched regex %q in %q", lineRegex, input)
	}

	failedMatches := make([]string, 0)
	for _, lineMatch := range lineMatches {
		// Look for the target valueRegex within this line
		valueMatched, err := regexp.Match(valueRegex, []byte(lineMatch))
		if err != nil {
			return err
		}
		if valueMatched {
			if expectMatch {
				// We expected to match and we did; no error
				return nil
			}
			// We matched but wasn't supposed to; return error
			return errors.Errorf("failed to verify %q didn't exist, but was in %q", valueRegex, lineMatch)
		}
		failedMatches = append(failedMatches, lineMatch)
	}
	if !expectMatch {
		// We didn't expect to match and didn't; no error
		return nil
	}
	// We expected to match, but didn't, return error
	return errors.Errorf("failed to find target value %q in lines %q", valueRegex, failedMatches)
}
