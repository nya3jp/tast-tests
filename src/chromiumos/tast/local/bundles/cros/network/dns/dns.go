// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dns

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
)

// DoHMode defines possible type of DNS-over-HTTPS.
type DoHMode int

const (
	// DoHOff is a mode that resolves DNS through plaintext.
	DoHOff DoHMode = iota
	// DoHAutomatic is a mode that automatically chose between plaintext and secure DNS.
	DoHAutomatic
	// DoHAlwaysOn is a mode that resolves DNS through secure DNS.
	DoHAlwaysOn
)

// Client defines the client resolving DNS.
type Client int

const (
	// System is a DNS client type for systems.
	System Client = iota
	// User is a DNS client type for users (e.g. cups, tlsdate).
	User
	// Chrome is a DNS client type with user 'chronos'.
	Chrome
	// Crostini is a DNS client type for Crostini.
	Crostini
	// ARC is a DNS client type for ARC.
	ARC
)

// ProxyTestCase contains test case for DNS proxy tests.
type ProxyTestCase struct {
	Client Client
}

// GoogleDoHProvider is the Google DNS-over-HTTPS provider.
const GoogleDoHProvider = "https://dns.google/dns-query"

// GetClientString get the string representation of a DNS client.
func GetClientString(c Client) string {
	switch c {
	case System:
		return "system"
	case User:
		return "user"
	case Chrome:
		return "Chrome"
	case Crostini:
		return "Crostini"
	case ARC:
		return "ARC"
	default:
		return ""
	}
}

// SetDoHMode updates Chrome OS setting to change DNS-over-HTTPS mode.
func SetDoHMode(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, mode DoHMode, dohProvider string) error {
	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osPrivacy")
	if err != nil {
		return errors.Wrap(err, "failed to get connection to OS Settings")
	}
	defer conn.Close()

	ac := uiauto.New(tconn)
	switch mode {
	case DoHOff:
		// Disable secure DNS, the UI might lag, keep trying until secure DNS is disabled.
		var secureDNSUnchecked = func(ctx context.Context) error {
			tb, err := ac.Info(ctx, nodewith.Role(role.ToggleButton).Name("Use secure DNS"))
			if err != nil {
				return errors.Wrap(err, "failed to find secure DNS toggle button")
			}
			if tb.Checked == checked.True {
				return errors.New("secure DNS toggle button is checked")
			}
			return nil
		}
		tb := nodewith.Role(role.ToggleButton).Name("Use secure DNS")
		if err := ac.LeftClickUntil(tb, secureDNSUnchecked)(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle secure DNS button")
		}
		return nil
	case DoHAutomatic:
		// Make sure secure DNS is turned on.
		if tbInfo, err := ac.Info(ctx, nodewith.Role(role.ToggleButton).Name("Use secure DNS")); err != nil {
			return errors.Wrap(err, "failed to find secure DNS toggle button")
		} else if tbInfo.Checked == checked.False {
			tb := nodewith.Role(role.ToggleButton).Name("Use secure DNS")
			if err := ac.LeftClick(tb)(ctx); err != nil {
				return errors.Wrap(err, "failed to toggle secure DNS button")
			}
		}
		rb := nodewith.Role(role.RadioButton).Name("With your current service provider")
		if err := ac.LeftClick(rb)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable automatic mode")
		}
		return nil
	case DoHAlwaysOn:
		// Make sure secure DNS is turned on.
		if tbInfo, err := ac.Info(ctx, nodewith.Role(role.ToggleButton).Name("Use secure DNS")); err != nil {
			return errors.Wrap(err, "failed to find secure DNS toggle button")
		} else if tbInfo.Checked == checked.False {
			tb := nodewith.Role(role.ToggleButton).Name("Use secure DNS")
			if err := ac.LeftClick(tb)(ctx); err != nil {
				return errors.Wrap(err, "failed to toggle secure DNS button")
			}
		}

		// Get a handle to the input keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		defer kb.Close()

		tf := nodewith.Role(role.TextField).Name("Enter custom provider")
		rg := nodewith.Role(role.RadioGroup)
		rbsInfo, err := ac.NodesInfo(ctx, nodewith.Role(role.RadioButton).Ancestor(rg))
		if err != nil {
			return errors.Wrap(err, "failed to get secure DNS radio buttons information")
		}
		var rbLocation coords.Rect
		var found = false
		for _, e := range rbsInfo {
			if e.Name != "With your current service provider" {
				rbLocation = e.Location
				found = true
				break
			}
		}
		if !found {
			return errors.Wrap(err, "failed to find secure DNS radio button")
		}

		if err := uiauto.Combine("enable DoH always on with a custom provider",
			// Input a custom DoH provider.
			ac.LeftClick(tf),
			kb.AccelAction("Ctrl+A"),
			kb.AccelAction("Backspace"),
			kb.TypeAction(dohProvider),
			// Click use current service provider radio button.
			ac.MouseClickAtLocation(0, rbLocation.CenterPoint()),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable DoH with a custom provider")
		}
		return nil
	}
	return nil
}

// QueryDNS resolves a domain through DNS with a specific client.
func QueryDNS(ctx context.Context, c Client, domain string) error {
	switch c {
	case System:
		return testexec.CommandContext(ctx, "dig", domain).Run()
	case User:
		return testexec.CommandContext(ctx, "sudo", "-u", "cups", "dig", domain).Run()
	case Chrome:
		return testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dig", domain).Run()
	case Crostini:
		// TODO(jasongustaman): Query DNS from Crostini.
		return nil
	case ARC:
		// TODO(jasongustaman): Query DNS from ARC.
		return nil
	}
	return errors.New("unknown client")
}

// TestQueryDNSProxy runs a set of test cases for DNS proxy.
func TestQueryDNSProxy(ctx context.Context, tcs []ProxyTestCase, domain string) []error {
	var errs []error
	for _, tc := range tcs {
		err := QueryDNS(ctx, tc.Client, domain)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "DNS query failed for %s", GetClientString(tc.Client)))
		}
	}
	return errs
}
