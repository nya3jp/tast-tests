// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VPNUI,
		Desc:         "Follows the user flow to create, connect, disconnect, and forget a VPN service via UI",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		// Do not use shillReset since we will need to also reset Chrome between
		// tests which is expensive.
		Fixture:      "chromeLoggedIn",
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func VPNUI(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/internet")
	if err != nil {
		s.Fatal("Failed to open the OS settings page: ", err)
	}
	defer conn.Close()

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer ew.Close()

	// Prepares VPN server.
	config := vpn.Config{
		Type:     vpn.TypeL2TPIPsec,
		AuthType: vpn.AuthTypePSK,
	}
	vpnConn, err := vpn.NewConnection(ctx, config)
	if err != nil {
		s.Fatal("Failed to create VPN connection: ", err)
	}
	defer vpnConn.Cleanup(ctx)
	if err := vpnConn.SetUpWithoutService(ctx); err != nil {
		s.Fatal("Failed to setup VPN server: ", err)
	}

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Open VPN dialog",
		ui.LeftClick(nodewith.Name("Add network connection").Role(role.Button)),
		ui.LeftClick(nodewith.NameContaining("Add built-in VPN").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to open VPN dialog: ", err)
	}

	if err := uiauto.Combine("Select VPN type",
		ui.LeftClick(nodewith.Name("Provider type").Role(role.PopUpButton)),
		ui.LeftClick(nodewith.Name("L2TP/IPsec").Role(role.ListBoxOption)),
	)(ctx); err != nil {
		s.Fatal("Failed to select VPN type: ", err)
	}

	// Inputs VPN properties via UI.
	svcName := "vpn-test-" + vpn.TypeL2TPIPsec
	var inputTextField = func(name, text string) {
		if err := ui.FocusAndWait(nodewith.Name(name).Role(role.TextField))(ctx); err != nil {
			s.Fatalf("Failed to focus %s: %v", name, err)
		}
		if err := ew.Type(ctx, text); err != nil {
			s.Fatalf("Failed to input %s: %v", name, err)
		}
	}
	inputTextField("Service name", svcName)
	inputTextField("Server hostname", vpnConn.Properties["Provider.Host"].(string))
	inputTextField("Username", vpnConn.Properties["L2TPIPsec.User"].(string))
	inputTextField("Password", vpnConn.Properties["L2TPIPsec.Password"].(string))
	inputTextField("Pre-shared key", vpnConn.Properties["L2TPIPsec.PSK"].(string))

	// Defers a cleanup to clear the profile in case of failure.
	defer func() {
		if err != vpn.RemoveVPNProfile(ctx, svcName) {
			s.Fatal("Failed to remove VPN service in cleanup: ", err)
		}
	}()

	// Clicks Connect and checks the "Connected" text on the VPN detail page.
	if err := uiauto.Combine("Connect VPN",
		ui.LeftClick(nodewith.Name("Connect").Role(role.Button)),
		ui.LeftClick(nodewith.Name("VPN").Role(role.Button)),
		ui.LeftClick(nodewith.NameContaining(svcName+", Details")),
		ui.WithTimeout(time.Second*5).WaitUntilExists(nodewith.Name("Connected").Role(role.StaticText)),
	)(ctx); err != nil {
		s.Fatal("Failed to connect VPN: ", err)
	}

	// Clicks Disconnect and checks the "Not Connected" text on the page.
	if err := uiauto.Combine("Disconnect VPN",
		ui.LeftClick(nodewith.Name("Disconnect").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Not Connected").Role(role.StaticText)),
	)(ctx); err != nil {
		s.Fatal("Failed to disconnect VPN: ", err)
	}

	// Clicks Forget, it should be navigated to the VPN list page and no service
	// should be shown.
	if err := uiauto.Combine("Forget VPN",
		ui.LeftClick(nodewith.Name("Forget").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("VPN").Role(role.Heading)),
		ui.Gone(nodewith.NameContaining(svcName)),
	)(ctx); err != nil {
		s.Fatal("Failed to forget VPN: ", err)
	}
}
