// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	// "chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/arcvpn"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/testing"

	"chromiumos/tast/common/testexec"
	"regexp"
	"strings"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ARCVPNConfigs,
		Desc:     "Host VPN configs are mirrored in ARC VPN properly",
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

func ARCVPNConfigs(ctx context.Context, s *testing.State) {
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
		Type:          vpn.TypeL2TPIPsecSwanctl,
		AuthType:      vpn.AuthTypePSK,
		MTU:           1499,
		Metered:       true,
		SearchDomains: []string{"foo", "bar"},
	}
	conn, cleanup, err := arcvpn.SetUpHostVPN(ctx, cleanupCtx, config)
	if err != nil {
		s.Fatal("Failed to setup host VPN: ", err)
	}
	defer cleanup()

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

	cmd := a.Command(ctx, "dumpsys", "wifi", "networks")
	o, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal(err, "failed to execute 'dumpsys wifi networks' commmand")
	}
	networks := strings.Split(string(o), "\n\n")
	for _, network := range networks {
		testing.ContextLog(ctx, "====")
		testing.ContextLog(ctx, network)
		matched, matchErr := regexp.Match(`transports=[A-Z|]*VPN`, o)
		if matchErr != nil {
			s.Fatal(err, "issue matching regexp for VPN transport")
		}
		if matched {
			testing.ContextLog(ctx, "!! found vpn")
		}
	}

	cmd = a.Command(ctx, "dumpsys", "wifi", "arc-networks")
	o, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal(err, "failed to execute 'dumpsys wifi arc-networks' commmand")
	}
	testing.ContextLog(ctx, string(o))
}
