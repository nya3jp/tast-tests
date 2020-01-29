// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ifconfigRe parses one adapter from the output of ifconfig.
var ifconfigRe = regexp.MustCompile("([^:]+): .*\n(?: +.*\n)*\n")

func listUpNetworkInterfaces(ctx context.Context) ([]string, error) {
	output, err := testexec.CommandContext(ctx, "ifconfig").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get interface list")
	}
	var interfaces []string
	match := ifconfigRe.FindAllSubmatch(output, -1)
	if match == nil {
		return nil, errors.Errorf("unable to parse interface list from %q", output)
	}
	for _, submatch := range match {
		interfaces = append(interfaces, string(submatch[1]))
	}
	return interfaces, nil
}

func enableNetworkInterface(ctx context.Context, iface string) error {
	if err := testexec.CommandContext(ctx, "ifconfig", iface, "up").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to enable network interface %q", iface)
	}
	return nil
}

func disableNetworkInterface(ctx context.Context, iface string) error {
	if err := testexec.CommandContext(ctx, "ifconfig", iface, "down").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to disable network interface %q", iface)
	}
	return nil
}

// DisableNetworkInterface disables a single network interface.
func DisableNetworkInterface(ctx context.Context, iface string, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	if err := disableNetworkInterface(ctx, iface); err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Disabled network interface %q", iface)

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		if err := enableNetworkInterface(ctx, iface); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Re-enabled network interface %q", iface)
		return nil
	})
}

// DisableNetworkInterfaces disables all network interfaces whose names matche
// a regexp.
func DisableNetworkInterfaces(ctx context.Context, pattern *regexp.Regexp, chain CleanupChain) (CleanupChain, error) {
	setupFailed, guard := SetupFailureGuard(chain)
	defer guard(ctx)

	upInterfaces, err := listUpNetworkInterfaces(ctx)
	if err != nil {
		return nil, err
	}

	innerChain := NewCleanupChain()
	for _, iface := range upInterfaces {
		if !pattern.MatchString(iface) {
			continue
		}
		innerChain, err = DisableNetworkInterface(ctx, iface, innerChain)
		if err != nil {
			return nil, err
		}
	}

	return SetupSucceeded(setupFailed, chain, func(ctx context.Context) error {
		return RunCleanupChain(ctx, innerChain)
	})
}

// DisableWiFiInterfaces disables all WiFi adapters and returns a callback to
// re-enable them.
func DisableWiFiInterfaces(ctx context.Context, chain CleanupChain) (CleanupChain, error) {
	var wifiInterfaceRe = regexp.MustCompile(".*wlan\\d+")
	return DisableNetworkInterfaces(ctx, wifiInterfaceRe, chain)
}
