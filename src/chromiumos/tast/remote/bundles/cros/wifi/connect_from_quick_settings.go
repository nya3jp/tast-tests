// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/chrome/uiauto/quicksettings"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConnectFromQuickSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that a user can connect to WiFi from the Quick Settings",
		Contacts: []string{
			"chadduffin@google.com",
			"cros-connectivity@google.com",
		},
		Attr: []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{
			"tast.cros.browser.ChromeService",
			"tast.cros.chrome.uiauto.quicksettings.QuickSettingsService",
			"tast.cros.ui.AutomationService",
			wificell.TFServiceName,
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "wificellFixtWithCapture",
		Params: []testing.Param{{
			Name: "open",
			Val: []ap.Option{
				ap.SSID("OpenNetworkSSID"),
				ap.Mode(ap.Mode80211a),
				ap.Channel(64),
				ap.SpectrumManagement(),
			},
		}},
	})
}

func ConnectFromQuickSettings(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	apInterface, err := tf.ConfigureAP(ctx, s.Param().([]ap.Option), base.NewConfigFactory())
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}

	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, apInterface); err != nil {
			s.Error("Failed to deconfigure AP: ", err)
		}
	}(ctx)

	ctx, cancel := tf.ReserveForDeconfigAP(ctx, apInterface)
	defer cancel()

	rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to create RPC client: ", err)
	}

	chrome := ui.NewChromeServiceClient(rpcClient.Conn)
	qs := quicksettings.NewQuickSettingsServiceClient(rpcClient.Conn)
	uiautomation := ui.NewAutomationServiceClient(rpcClient.Conn)

	if _, err = chrome.New(ctx, &ui.NewRequest{
		LoginMode: ui.LoginMode_LOGIN_MODE_GUEST_LOGIN,
	}); err != nil {
		s.Fatal("Failed to open Chrome on the DUT: ", err)
	}

	if _, err = qs.NavigateToNetworkDetailedView(ctx, &emptypb.Empty{}); err != nil {
		s.Error("Failed to navigate to the detailed Network within Quick Settings: ", err)
	}

	networkFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: "OpenNetworkSSID"}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}
	if _, err := uiautomation.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: networkFinder}); err != nil {
		s.Fatal("Failed to find the network button: ", err)
	}
	if _, err := uiautomation.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: networkFinder}); err != nil {
		s.Fatal("Failed to click the network button: ", err)
	}
	// if _, err = chrome.Close(ctx, &emptypb.Empty{}); err != nil {
	// 	s.Error("Failed to close Chrome on the DUT: ", err)
	// }
	if err = rpcClient.Close(ctx); err != nil {
		s.Error("Failed to close RPC client: ", err)
	}
}
