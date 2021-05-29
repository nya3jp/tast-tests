// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dns

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
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

	switch mode {
	case DoHOff:
		// Disable secure DNS, the UI might lag, keep trying until secure DNS is disabled.
		return testing.Poll(ctx, func(ctx context.Context) error {
			tb, err := ui.FindWithTimeout(
				ctx,
				tconn,
				ui.FindParams{Role: ui.RoleTypeToggleButton, Name: "Use secure DNS"},
				5*time.Second,
			)
			if err != nil {
				return errors.Wrap(err, "failed to find secure DNS toggle button")
			}
			defer tb.Release(ctx)

			if tb.Checked == ui.CheckedStateFalse {
				return nil
			}
			if err := tb.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to toggle secure DNS button")
			}
			return errors.New("failed to toggle secure DNS button")
		}, &testing.PollOptions{Timeout: 60 * time.Second})
	case DoHAutomatic:
		tb, err := ui.FindWithTimeout(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeToggleButton, Name: "Use secure DNS"},
			5*time.Second,
		)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS toggle button")
		}
		defer tb.Release(ctx)

		// Make sure secure DNS is turned on.
		if tb.Checked == ui.CheckedStateFalse {
			if err := tb.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to toggle secure DNS button")
			}
		}

		// Find and click "With your current service provider" button.
		rb, err := ui.FindWithTimeout(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeRadioButton, Name: "With your current service provider"},
			5*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS automatic radio button")
		}
		defer rb.Release(ctx)

		if err := rb.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to left click secure DNS radio button")
		}
		return nil
	case DoHAlwaysOn:
		tb, err := ui.FindWithTimeout(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeToggleButton, Name: "Use secure DNS"},
			5*time.Second,
		)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS toggle button")
		}
		defer tb.Release(ctx)

		// Make sure secure DNS is turned on.
		if tb.Checked == ui.CheckedStateFalse {
			if err := tb.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to toggle secure DNS button")
			}
		}

		// Input a custom DoH provider.
		tf, err := ui.FindWithTimeout(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeTextField, Name: "Enter custom provider"},
			5*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS provider text field")
		}
		defer tf.Release(ctx)
		if err := tf.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to left click secure DNS provider text field")
		}

		// Get a handle to the input keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		defer kb.Close()
		if err := kb.Accel(ctx, "Ctrl+A"); err != nil {
			return errors.Wrap(err, "failed to clear secure DNS provider text")
		}
		if err := kb.Accel(ctx, "Backspace"); err != nil {
			return errors.Wrap(err, "failed to clear secure DNS provider text")
		}
		if err := kb.Type(ctx, dohProvider); err != nil {
			return errors.Wrap(err, "failed to type secure DNS provider")
		}

		// Find and click "use custom DNS provider" button.
		rg, err := ui.FindWithTimeout(
			ctx,
			tconn,
			ui.FindParams{Role: ui.RoleTypeRadioGroup},
			5*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find secure DNS radio group")
		}
		defer rg.Release(ctx)
		rbs, err := rg.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeRadioButton})
		if err != nil {
			return errors.Wrap(err, "failed to get secure DNS radio group descendants")
		}
		defer rbs.Release(ctx)

		var rb *ui.Node
		for _, e := range rbs {
			if e.Name != "With your current service provider" {
				rb = e
			}
		}
		if rb == nil {
			return errors.Wrap(err, "failed to find secure DNS radio button")
		}
		if err := rb.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to left click secure DNS radio button")
		}
		return nil
	}
	return nil
}

// QueryDNS resolves a domain through DNS with a specific client.
func QueryDNS(ctx context.Context, c Client, a *arc.ARC, cont *vm.Container, domain string) error {
	switch c {
	case System:
		return testexec.CommandContext(ctx, "dig", domain).Run()
	case User:
		return testexec.CommandContext(ctx, "sudo", "-u", "cups", "dig", domain).Run()
	case Chrome:
		return testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dig", domain).Run()
	case Crostini:
		return cont.Command(ctx, "dig", domain).Run()
	case ARC:
		return a.Command(ctx, "dumpsys", "wifi", "tools", "dns", domain).Run()
	}
	return errors.New("unknown client")
}

// InstallDigInContainer installs dig in container.
func InstallDigInContainer(ctx context.Context, cont *vm.Container) error {
	// Check whether dig is preinstalled or not.
	if err := cont.Command(ctx, "dig", "-v").Run(); err == nil {
		return nil
	}

	// Run command sudo apt update in container.
	if err := cont.Command(ctx, "sudo", "apt", "update").Run(); err != nil {
		return errors.Wrap(err, "failed to run command sudo apt update in container")
	}

	// Run command sudo apt install dnsutils in container.
	if err := cont.Command(ctx, "sudo", "DEBIAN_FRONTEND=noninteractive", "apt-get", "-y", "install", "dnsutils").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run command sudo apt install dnsutils in container")
	}

	// Run command dig -v and check the output to make sure vim has been installed successfully.
	if err := cont.Command(ctx, "dig", "-v").Run(); err != nil {
		return errors.Wrap(err, "failed to install dig in container")
	}
	return nil
}
