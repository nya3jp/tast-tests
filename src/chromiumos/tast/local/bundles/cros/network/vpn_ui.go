// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	localping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VPNUI,
		Desc:         "Follows the user flow to create, connect, disconnect, and forget a VPN service via UI",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "vpnShillResetWithChromeLoggedIn",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Params: []testing.Param{{
			Name: "l2tp_ipsec_cert",
			Val: vpn.Config{
				Type:     vpn.TypeL2TPIPsec,
				AuthType: vpn.AuthTypeCert,
			},
		}, {
			Name: "l2tp_ipsec_psk",
			Val: vpn.Config{
				Type:     vpn.TypeL2TPIPsec,
				AuthType: vpn.AuthTypePSK,
			},
		}, {
			Name: "wireguard",
			Val: vpn.Config{
				Type:     vpn.TypeWireGuard,
				AuthType: vpn.AuthTypePSK,
			},
			ExtraSoftwareDeps: []string{"wireguard"},
		}},
	})
}

func VPNUI(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(vpn.FixtureEnv).Cr
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
	config := s.Param().(vpn.Config)
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

	// Inputs VPN properties via UI.
	svcName := "vpn-test-" + config.Type
	if len(config.AuthType) > 0 {
		svcName += "-" + config.AuthType
	}

	// Configures service on the VPN dialog page.
	v := vpnDialogConfigger{ui, ew, config, vpnConn, svcName}
	if err := v.config(ctx); err != nil {
		s.Fatal("Failed to configure on VPN dialog: ", err)
	}

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

	// Pings server gateway to make sure VPN is connected. This is required since
	// some VPN services (e.g., WireGuard) will show connected even if we have a
	// wrong configuration.
	pr := localping.NewLocalRunner()
	if err := vpn.ExpectPingSuccess(ctx, pr, vpnConn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s: %v", vpnConn.Server.OverlayIP, err)
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

type vpnDialogConfigger struct {
	ui      *uiauto.Context
	ew      *input.KeyboardEventWriter
	cfg     vpn.Config
	conn    *vpn.Connection
	svcName string
}

func (v *vpnDialogConfigger) inputTextField(ctx context.Context, name, value string) error {
	if err := v.ui.FocusAndWait(nodewith.Name(name).Role(role.TextField))(ctx); err != nil {
		return errors.Wrapf(err, "failed to focus %s", name)
	}
	if err := v.ew.Type(ctx, value); err != nil {
		return errors.Wrapf(err, "failed to input %s", name)
	}
	return nil
}

func (v *vpnDialogConfigger) selectListOption(ctx context.Context, name, value string) error {
	return uiauto.Combine("Select "+name,
		v.ui.LeftClick(nodewith.Name(name).Role(role.PopUpButton)),
		v.ui.LeftClick(nodewith.Name(value).Role(role.ListBoxOption)),
	)(ctx)
}

func (v *vpnDialogConfigger) config(ctx context.Context) error {
	if err := v.inputTextField(ctx, "Service name", v.svcName); err != nil {
		return err
	}
	switch v.cfg.Type {
	case vpn.TypeL2TPIPsec:
		return v.configL2TPIPsec(ctx)
	case vpn.TypeWireGuard:
		return v.configWireGuard(ctx)
	default:
		return errors.Errorf("invalid VPN type %s", v.cfg.Type)
	}
}

func (v *vpnDialogConfigger) configL2TPIPsec(ctx context.Context) error {
	if err := v.selectListOption(ctx, "Provider type", "L2TP/IPsec"); err != nil {
		return errors.Wrap(err, "failed to select VPN type")
	}
	if err := v.inputTextField(ctx, "Server hostname", v.conn.Properties["Provider.Host"].(string)); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Username", v.conn.Properties["L2TPIPsec.User"].(string)); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Password", v.conn.Properties["L2TPIPsec.Password"].(string)); err != nil {
		return err
	}

	switch v.cfg.AuthType {
	case vpn.AuthTypeCert:
		if err := v.selectListOption(ctx, "Authentication type", "User certificate"); err != nil {
			return errors.Wrap(err, "failed to select authentication type")
		}
		// User cert and server CA should be selected by default.
	case vpn.AuthTypePSK:
		// Authentication type is default to "Pre-shared key".
		if err := v.inputTextField(ctx, "Pre-shared key", v.conn.Properties["L2TPIPsec.PSK"].(string)); err != nil {
			return err
		}
	default:
		return errors.Errorf("unknown auth type %s", v.cfg.AuthType)
	}
	return nil
}

func (v *vpnDialogConfigger) configWireGuard(ctx context.Context) error {
	if err := v.selectListOption(ctx, "Provider type", "WireGuard"); err != nil {
		return errors.Wrap(err, "failed to select VPN type")
	}

	staticIPConfig := v.conn.Properties["StaticIPConfig"].(map[string]interface{})
	peer := v.conn.Properties["WireGuard.Peers"].([]map[string]string)[0]
	if err := v.inputTextField(ctx, "Client IP address", staticIPConfig["Address"].(string)); err != nil {
		return err
	}
	if err := v.selectListOption(ctx, "Key", "I have a keypair"); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Private key", v.conn.Properties["WireGuard.PrivateKey"].(string)); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Public key", peer["PublicKey"]); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Preshared key", peer["PresharedKey"]); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Endpoint", peer["Endpoint"]); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Allowed IPs", peer["AllowedIPs"]); err != nil {
		return err
	}
	return nil
}
