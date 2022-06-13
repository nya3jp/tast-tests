// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dns

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/shill"
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

// ProxyTestCase contains test case for DNS proxy tests.
type ProxyTestCase struct {
	Client    Client
	ExpectErr bool
}

// GoogleDoHProvider is the Google DNS-over-HTTPS provider.
const GoogleDoHProvider = "https://dns.google/dns-query"

// DigProxyIPRE is the regular expressions for DNS proxy IP inside dig output.
var DigProxyIPRE = regexp.MustCompile(`SERVER: 100.115.92.\d+#53`)

// DigIPRE returns a regular expression for matching |ns| in dig output.
func DigIPRE(ns string) (*regexp.Regexp, error) {
	return regexp.Compile("SERVER: " + ns + "#53")
}

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

// SetDoHMode updates ChromeOS setting to change DNS-over-HTTPS mode.
func SetDoHMode(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, mode DoHMode, dohProvider string) error {
	conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osPrivacy")
	if err != nil {
		return errors.Wrap(err, "failed to get connection to OS Settings")
	}
	defer conn.Close()

	ac := uiauto.New(tconn)

	// Toggle secure DNS, the UI might lag, keep trying until secure DNS is toggled to the expected state.
	leftClickAc := ac.WithInterval(2 * time.Second)
	var toggleSecureDNS = func(ctx context.Context, check checked.Checked) error {
		tb := nodewith.Role(role.ToggleButton).Name("Use secure DNS")
		var secureDNSChecked = func(ctx context.Context) error {
			tbInfo, err := ac.Info(ctx, tb)
			if err != nil {
				return errors.Wrap(err, "failed to find secure DNS toggle button")
			}
			if tbInfo.Checked != check {
				return errors.Errorf("secure DNS toggle button checked: %s", check)
			}
			return nil
		}
		if err := leftClickAc.LeftClickUntil(tb, secureDNSChecked)(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle secure DNS button")
		}
		return nil
	}

	switch mode {
	case DoHOff:
		if err := toggleSecureDNS(ctx, checked.False); err != nil {
			return err
		}
		break
	case DoHAutomatic:
		if err := toggleSecureDNS(ctx, checked.True); err != nil {
			return err
		}

		rb := nodewith.Role(role.RadioButton).Name("With your current service provider")
		if err := ac.LeftClick(rb)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable automatic mode")
		}
		break
	case DoHAlwaysOn:
		if err := toggleSecureDNS(ctx, checked.True); err != nil {
			return err
		}

		// Get a handle to the input keyboard.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		defer kb.Close()

		m, err := input.Mouse(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get mouse")
		}
		defer m.Close()

		// On some devices, the text field for the provider might be hidden by the bottom bar.
		// Scroll down then focus on the text field.
		if err := m.ScrollDown(); err != nil {
			return errors.Wrap(err, "failed to scroll down")
		}

		// Find secure DNS text field through its parent.
		gcs, err := ac.NodesInfo(ctx, nodewith.Role(role.GenericContainer))
		if err != nil {
			return errors.Wrap(err, "failed to get generic container nodes")
		}
		nth := -1
		for i, e := range gcs {
			if attr, ok := e.HTMLAttributes["id"]; ok && attr == "secureDnsInput" {
				nth = i
				break
			}
		}
		if nth < 0 {
			return errors.Wrap(err, "failed to find secure DNS text field")
		}
		tf := nodewith.Role(role.TextField).Ancestor(nodewith.Role(role.GenericContainer).Nth(nth))
		if err := ac.FocusAndWait(tf)(ctx); err != nil {
			return errors.Wrap(err, "failed to focus on the text field")
		}

		rg := nodewith.Role(role.RadioGroup)
		if err := ac.WaitForLocation(rg)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for radio group")
		}
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
			// Click use current service provider radio button.
			ac.MouseClickAtLocation(0, rbLocation.CenterPoint()),
			// Input a custom DoH provider.
			ac.LeftClick(tf),
			kb.AccelAction("Ctrl+A"),
			kb.AccelAction("Backspace"),
			kb.TypeAction(dohProvider),
			kb.AccelAction("Enter"),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable DoH with a custom provider")
		}
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if m, err := getDoHMode(ctx); err != nil {
			return err
		} else if m != mode {
			return errors.New("failed to get the correct DoH mode")
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return err
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

// TestQueryDNSProxy runs a set of test cases for DNS proxy.
func TestQueryDNSProxy(ctx context.Context, tcs []ProxyTestCase, a *arc.ARC, cont *vm.Container, domain string) []error {
	var errs []error
	for _, tc := range tcs {
		err := QueryDNS(ctx, tc.Client, a, cont, domain)
		if err != nil && !tc.ExpectErr {
			errs = append(errs, errors.Wrapf(err, "DNS query failed for %s", GetClientString(tc.Client)))
		}
		if err == nil && tc.ExpectErr {
			errs = append(errs, errors.Errorf("successful DNS query for %s, but expected failure", GetClientString(tc.Client)))
		}
	}
	return errs
}

// InstallDigInContainer installs dig in container.
func InstallDigInContainer(ctx context.Context, cont *vm.Container) error {
	// Check whether dig is preinstalled or not.
	if err := cont.Command(ctx, "dig", "-v").Run(); err == nil {
		return nil
	}

	// Run command sudo apt update in container. Ignore the error because this might fail for unrelated reasons.
	cont.Command(ctx, "sudo", "apt", "update").Run(testexec.DumpLogOnError)

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

// getDoHProviders returns the current DNS-over-HTTPS providers.
func getDoHProviders(ctx context.Context) (map[string]interface{}, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager object")
	}

	props, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	out, err := props.Get(shillconst.ManagerPropertyDOHProviders)
	if err != nil {
		return nil, err
	}
	providers, ok := out.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("property %s is not a map of string to interface: %q", shillconst.ManagerPropertyDOHProviders, out)
	}
	return providers, nil
}

// getDoHMode returns the current DNS-over-HTTPS mode.
func getDoHMode(ctx context.Context) (DoHMode, error) {
	providers, err := getDoHProviders(ctx)
	if err != nil || len(providers) == 0 {
		return DoHOff, err
	}
	for _, ns := range providers {
		if ns == "" {
			continue
		}
		return DoHAutomatic, nil
	}
	return DoHAlwaysOn, nil
}

// DigMatch runs dig to check name resolution works and verifies the expected server was used.
func DigMatch(ctx context.Context, re *regexp.Regexp, match bool) error {
	out, err := testexec.CommandContext(ctx, "dig", "google.com").Output()
	if err != nil {
		return errors.Wrap(err, "dig failed")
	}
	if re.MatchString(string(out)) != match {
		return errors.New("dig used unexpected nameserver")
	}
	return nil
}

// DigToMatch runs dig to a specific nameserver, checks name resolution works and verifies that server was used.
func DigToMatch(ctx context.Context, ns string, match bool) error {
	re, err := DigIPRE(ns)
	if err != nil {
		return err
	}
	out, err := testexec.CommandContext(ctx, "dig", "google.com", "@"+ns).Output()
	if err != nil {
		return errors.Wrap(err, "dig failed")
	}
	if re.MatchString(string(out)) != match {
		return errors.New("dig used unexpected nameserver")
	}
	return nil
}
