// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"
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
		Func:     ARCVPNConfigs,
		Desc:     "Host VPN configs are reflected properly in ARC VPN",
		Contacts: []string{"cassiewang@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "shillResetWithArcBooted",
		Params: []testing.Param{{
			Val:               "p",
			ExtraSoftwareDeps: []string{"android_p", "wireguard"},
		}, {
			Name:              "vm",
			Val:               "vm",
			ExtraSoftwareDeps: []string{"android_vm", "wireguard"},
		}},
	})
}

// ARCVPNConfigs tests that a few specific config fields from the host VPN are passed and set on
// the mirrored ARC VPN correctly.
func ARCVPNConfigs(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a
	// few seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 6*time.Second)
	defer cancel()

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
	metered := false
	searchDomains := []string{"foo1", "bar1"}
	config1 := vpn.Config{
		Type:          vpn.TypeWireGuard,
		Metered:       metered,
		SearchDomains: searchDomains,
		MTU:           576,
	}
	conn1, cleanup1, err := arcvpn.SetUpHostVPNWithConfig(ctx, cleanupCtx, config1)
	if err != nil {
		s.Fatal("Failed to setup host VPN: ", err)
	}
	defer cleanup1()
	if _, err := conn1.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	}
	if err := arcvpn.CheckARCVPNState(ctx, a, true); err != nil {
		s.Fatal("Failed to start ArcHostVpnService: ", err)
	}
	pr := localping.NewLocalRunner()
	if err := vpn.ExpectPingSuccess(ctx, pr, conn1.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn1.Server.OverlayIP, err)
	}
	if err := arcvpn.ExpectARCPingSuccess(ctx, a, "vpn", conn1.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s from ARC over 'vpn': %v", conn1.Server.OverlayIP, err)
	}
	cmd := a.Command(ctx, "dumpsys", "wifi", "networks", "transport", "vpn")
	o, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal(err, "Failed to execute 'dumpsys wifi networks transport vpn'")
	}
	oStr := string(o)
	// On P, the VpnService.Builder#setMetered API isn't available for us to override the value
	// and VPNs are considered metered by default.
	arc := s.Param().(string)
	if arc == "p" {
		metered = true
	}
	if err := checkMatch(oStr, `capabilities=.*`, `NOT_METERED`, !metered); err != nil {
		s.Fatal("Failed to verify capabilities on ARC VPN network: ", err)
	}
	for _, domain := range searchDomains {
		if err := checkMatch(oStr, `domains=.*`, domain, true); err != nil {
			s.Fatal("Failed to verify search domains on ARC VPN network: ", err)
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
		s.Fatal(err, "Failed to execute 'ifconfig tun0'")
	}
	if err := checkMatch(string(o), `MTU:.*`, `576`, true); err != nil {
		s.Fatal("Failed to verify MTU on ARC VPN network: ", err)
	}

	// Disconnect from our first connection.
	if err := conn1.Disconnect(ctx); err != nil {
		s.Error("Failed to disconnect VPN: ", err)
	}
	if err := arcvpn.CheckARCVPNState(ctx, a, false); err != nil {
		s.Fatal("ArcHostVpnService should be stopped, but isn't: ", err)
	}
	if err := arcvpn.ExpectARCPingSuccess(ctx, a, "vpn", conn1.Server.OverlayIP); err == nil {
		s.Fatalf("Expected unable to ping %s from ARC over 'vpn', but was reachable", conn1.Server.OverlayIP)
	}

	// Connect with a different config and verify values. Use values that are different from
	// the first connection's config's values to ensure we didn't just get lucky with some
	// default values.
	metered = true
	searchDomains = []string{"foo2", "bar2"}
	config2 := vpn.Config{
		Type:          vpn.TypeWireGuard,
		Metered:       metered,
		SearchDomains: searchDomains,
		MTU:           1280,
	}
	conn2, cleanup2, err := arcvpn.SetUpHostVPNWithConfig(ctx, cleanupCtx, config2)
	if err != nil {
		s.Fatal("Failed to setup host VPN: ", err)
	}
	defer cleanup2()
	if _, err := conn2.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	}
	if err := arcvpn.CheckARCVPNState(ctx, a, true); err != nil {
		s.Fatal("Failed to start ArcHostVpnService: ", err)
	}
	pr = localping.NewLocalRunner()
	if err := vpn.ExpectPingSuccess(ctx, pr, conn2.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping from host %s: %v", conn2.Server.OverlayIP, err)
	}
	if err := arcvpn.ExpectARCPingSuccess(ctx, a, "vpn", conn2.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s from ARC over 'vpn': %v", conn2.Server.OverlayIP, err)
	}
	cmd = a.Command(ctx, "dumpsys", "wifi", "networks", "transport", "vpn")
	o, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal(err, "Failed to execute 'dumpsys wifi networks transport vpn'")
	}
	oStr = string(o)
	// On P, the VpnService.Builder#setMetered API isn't available for us to override the value
	// and VPNs are considered metered by default.
	if arc == "p" {
		metered = true
	}
	if err := checkMatch(oStr, `capabilities=.*`, `NOT_METERED`, !metered); err != nil {
		s.Fatal("Failed to verify capabilities on ARC VPN network: ", err)
	}
	for _, domain := range searchDomains {
		if err := checkMatch(oStr, `domains=.*`, domain, true); err != nil {
			s.Fatal("Failed to verify search domains on ARC VPN network: ", err)
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
		s.Fatal(err, "Failed to execute 'ifconfig tun0'")
	}
	if err := checkMatch(string(o), `MTU:.*`, `1280`, true); err != nil {
		s.Fatal("Failed to verify MTU on ARC VPN network: ", err)
	}
}

// checkMatch will check that there is some 'lineRegex' within 'input'. And within the submatch
// that matched the 'lineRegex', we should/shouldn't 'match' a 'value'.
//
// For example, given:
//
//	input:
//	        "foo=hello\n
//	        bar=world jupiter"
//	lineRegex:
//	        `bar=.*`
//	value:
//	        `world`
//	match:
//	        true
//
// In the example above, the 'lineRegex' would find the submatch 'bar=world jupiter'. And we expect
// to find 'world' somewhere within that submatch. The 'lineRegex' must be found within the given
// 'input'. Whether the 'value' is expected in the submatch is up to 'match'.
func checkMatch(input, lineRegex, value string, expectMatch bool) error {
	// Narrow down the input to the target lineRegex
	lineRe := regexp.MustCompile(lineRegex)
	lineMatches := lineRe.FindStringSubmatch(input)
	if len(lineMatches) == 0 {
		return errors.Errorf("failed to find any lines that matched regex %q in %q", lineRegex, input)
	}

	failedMatches := make([]string, 1)
	for _, lineMatch := range lineMatches {
		// Look for the target value within this line submatch
		valueMatched, err := regexp.Match(value, []byte(lineMatch))
		if err != nil {
			return err
		}
		if valueMatched {
			if expectMatch {
				// We expected to match and we did; no error
				return nil
			}
			// We matched but wasn't supposed to; return error
			return errors.Errorf("did not expect to find %q, but was in %q", value, lineMatch)
		}
		failedMatches = append(failedMatches, lineMatch)
	}
	if !expectMatch {
		// We didn't expect to match and didn't; no error
		return nil
	}
	// We expected to match, but didn't, return error
	return errors.Errorf("failed to find target value %q in line submatches %q", value, failedMatches)
}
