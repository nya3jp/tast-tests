// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/network/routing"
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
			Name: "ikev2_cert",
			Val: vpn.Config{
				Type:     vpn.TypeIKEv2,
				AuthType: vpn.AuthTypeCert,
			},
			ExtraSoftwareDeps: []string{"ikev2"},
		}, {
			Name: "ikev2_eap",
			Val: vpn.Config{
				Type:     vpn.TypeIKEv2,
				AuthType: vpn.AuthTypeEAP,
			},
			ExtraSoftwareDeps: []string{"ikev2"},
		}, {
			Name: "ikev2_psk",
			Val: vpn.Config{
				Type:     vpn.TypeIKEv2,
				AuthType: vpn.AuthTypePSK,
			},
			ExtraSoftwareDeps: []string{"ikev2"},
		}, {
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
			Name: "openvpn",
			Val: vpn.Config{
				Type:                   vpn.TypeOpenVPN,
				AuthType:               vpn.AuthTypeCert,
				OpenVPNUseUserPassword: true,
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

	testing.ContextLog(ctx, "Setting keyboard layout to English (US)")
	imePrefix, err := ime.Prefix(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the ime prefix: ", err)
	}
	if err := ime.AddAndSetInputMethod(ctx, tconn, imePrefix+ime.EnglishUS.ID); err != nil {
		s.Fatal("Failed to set keyboard to en-US: ", err)
	}

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
	if err := routing.ExpectPingSuccessWithTimeout(ctx, vpnConn.Server.OverlayIPv4, "chronos", 10*time.Second); err != nil {
		s.Fatalf("Failed to ping %s: %v", vpnConn.Server.OverlayIPv4, err)
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
	btn := nodewith.Name(name).Role(role.ComboBoxSelect)
	return uiauto.Combine("Select "+name,
		v.ui.MakeVisible(btn),
		v.ui.LeftClick(btn),
		v.ui.LeftClick(nodewith.Name(value).Role(role.ListBoxOption)),
	)(ctx)
}

func (v *vpnDialogConfigger) config(ctx context.Context) error {
	if err := v.inputTextField(ctx, "Service name", v.svcName); err != nil {
		return err
	}
	switch v.cfg.Type {
	case vpn.TypeIKEv2:
		return v.configIKEv2(ctx)
	case vpn.TypeL2TPIPsec:
		return v.configL2TPIPsec(ctx)
	case vpn.TypeOpenVPN:
		return v.configOpenVPN(ctx)
	case vpn.TypeWireGuard:
		return v.configWireGuard(ctx)
	default:
		return errors.Errorf("invalid VPN type %s", v.cfg.Type)
	}
}

func (v *vpnDialogConfigger) configIKEv2(ctx context.Context) error {
	if err := v.selectListOption(ctx, "Provider type", "IPsec (IKEv2)"); err != nil {
		return errors.Wrap(err, "failed to select VPN type")
	}
	if err := v.inputTextField(ctx, "Server hostname", v.conn.Properties["Provider.Host"].(string)); err != nil {
		return err
	}
	switch v.cfg.AuthType {
	case vpn.AuthTypeCert:
		// User cert and server CA are selected by default.
		if err := v.selectListOption(ctx, "Authentication type", "User certificate"); err != nil {
			return errors.Wrap(err, "failed to select authentication type")
		}
		if err := v.inputTextField(ctx, "Remote identity (optional)", v.conn.Properties["IKEv2.RemoteIdentity"].(string)); err != nil {
			return err
		}
	case vpn.AuthTypeEAP:
		// Server CA is selected by default.
		if err := v.selectListOption(ctx, "Authentication type", "Username and password"); err != nil {
			return errors.Wrap(err, "failed to select authentication type")
		}
		if err := v.inputTextField(ctx, "Username", v.conn.Properties["EAP.Identity"].(string)); err != nil {
			return err
		}
		if err := v.inputTextField(ctx, "Password", v.conn.Properties["EAP.Password"].(string)); err != nil {
			return err
		}
	case vpn.AuthTypePSK:
		if err := v.selectListOption(ctx, "Authentication type", "Pre-shared key"); err != nil {
			return errors.Wrap(err, "failed to select authentication type")
		}
		if err := v.inputTextField(ctx, "Pre-shared key", v.conn.Properties["IKEv2.PSK"].(string)); err != nil {
			return err
		}
		if err := v.inputTextField(ctx, "Local identity (optional)", v.conn.Properties["IKEv2.LocalIdentity"].(string)); err != nil {
			return err
		}
		if err := v.inputTextField(ctx, "Remote identity (optional)", v.conn.Properties["IKEv2.RemoteIdentity"].(string)); err != nil {
			return err
		}
	default:
		return errors.Errorf("unknown auth type %s", v.cfg.AuthType)
	}
	return nil
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

func (v *vpnDialogConfigger) configOpenVPN(ctx context.Context) error {
	if err := v.selectListOption(ctx, "Provider type", "OpenVPN"); err != nil {
		return errors.Wrap(err, "failed to select VPN type")
	}
	if err := v.inputTextField(ctx, "Server hostname", v.conn.Properties["Provider.Host"].(string)); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Username", v.conn.Properties["OpenVPN.User"].(string)); err != nil {
		return err
	}
	if err := v.inputTextField(ctx, "Password", v.conn.Properties["OpenVPN.Password"].(string)); err != nil {
		return err
	}

	// Server CA is selected by default. Only need to select user cert.
	certName := "chromelab-wifi-testbed-root.mtv.google.com [chromelab-wifi-testbed-client.mtv.google.com]"
	if err := v.selectListOption(ctx, "User certificate", certName); err != nil {
		return errors.Wrap(err, "failed to select user certificate")
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
