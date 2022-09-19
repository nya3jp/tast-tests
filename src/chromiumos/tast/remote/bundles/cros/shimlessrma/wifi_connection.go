// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains integration tests for Shimless RMA SWA.
package shimlessrma

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/shimlessrma/rmaweb"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

const (
	offlineOperationTimeout = 2 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WifiConnection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test wifi connection will be forgotten after Shimless RMA",
		Contacts: []string{
			"yanghenry@google.com",
			"chromeos-engprod-syd@google.com",
		},
		Attr: []string{"group:shimless_rma", "shimless_rma_experimental"},
		Vars: []string{"router"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.browser.ChromeService",
			"tast.cros.shimlessrma.AppService",
			wificell.TFServiceName,
		},
		Fixture: fixture.NormalMode,
		Timeout: 10 * time.Minute,
	})
}

func WifiConnection(ctx context.Context, s *testing.State) {
	firmwareHelper := s.FixtValue().(*fixture.Value).Helper
	dut := firmwareHelper.DUT
	key := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")
	if err := firmwareHelper.RequireServo(ctx); err != nil {
		s.Fatal("Fail to init servo: ", err)
	}

	s.Log("Wifi config start")
	var tfOpts []wificell.TFOption
	if router, ok := s.Var("router"); ok && router != "" {
		tfOpts = append(tfOpts, wificell.TFRouter(router))
	}
	tf, err := wificell.NewTestFixture(ctx, ctx, dut, s.RPCHint(), tfOpts...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	s.Log("wificell.NewTestFixture succeeds")

	// Other options may work as well.
	// We only use one valid config since we focus on Shimless RMA rather than network config.
	options := []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)}
	apIface, err := tf.ConfigureAP(ctx, options, nil)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}

	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, apIface); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}(ctx)

	ctx, cancel := tf.ReserveForDeconfigAP(ctx, apIface)
	defer cancel()
	s.Logf("AP setup done. AP name is %s", apIface.Config().SSID)

	// Got idea from http://b/182202226#comment52
	// It allows internet connection on AP.
	// Shimless RMA needs Internet Access before moving to the next step.
	enableInternetScript := "iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE;" +
		"iptables -A INPUT -i managed0 -j ACCEPT;" +
		"iptables -A INPUT -i eth0 -m state --state ESTABLISHED,RELATED -j ACCEPT;" +
		"iptables -A OUTPUT -j ACCEPT;" +
		"echo 1 > /proc/sys/net/ipv4/ip_forward"
	apConn := tf.APConn()
	if err := apConn.CommandContext(ctx, "sh", "-c", enableInternetScript).Run(); err != nil {
		s.Fatal("Fail to run iptables commands to enable Internet access: ", err)
	}

	disableInternetScript := "iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE;" +
		"iptables -D INPUT -i managed0 -j ACCEPT;" +
		"iptables -D INPUT -i eth0 -m state --state ESTABLISHED,RELATED -j ACCEPT;" +
		"iptables -D OUTPUT -j ACCEPT;" +
		"echo 1 > /proc/sys/net/ipv4/ip_forward"

	defer func() {
		if err := apConn.CommandContext(ctx, "sh", "-c", disableInternetScript).Run(); err != nil {
			s.Fatal("Fail to run iptables commands to disable Internet access: ", err)
		}
	}()

	uiHelper, err := rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, false)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}

	if err := uiHelper.PrepareOfflineTest(ctx); err != nil {
		s.Fatal("Fail to prepare offline test: ", err)
	}
	s.Log("Prepare offline test successfully")

	// Limit the timeout for the preparation steps.
	offlineCtx, cancel := context.WithTimeout(ctx, offlineOperationTimeout)
	defer cancel()

	// Ignore any error in the following gRPC call,
	// because ethernet is turned off in that call.
	// As a result, we always get gRPC connection error.
	_ = uiHelper.WelcomeAndNetworkPageOperationOffline(offlineCtx, apIface.Config().SSID)

	s.Log("Offline time is completed")

	uiHelper, err = rmaweb.NewUIHelper(ctx, dut, firmwareHelper, s.RPCHint(), key, false)
	if err != nil {
		s.Fatal("Fail to initialize RMA Helper: ", err)
	}

	if err := uiHelper.VerifyOfflineOperationSuccess(ctx); err != nil {
		s.Fatal("Offline operation failed: ", err)
	}
	s.Log("offline operation succeed")

	if err := uiHelper.VerifyWifiIsForgotten(ctx); err != nil {
		s.Fatal("Failed to forget wifi: ", err)
	}
}
