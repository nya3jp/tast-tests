// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ifconfigRE parses one adapter from the output of ifconfig.
var ifconfigRE = regexp.MustCompile("([^:]+): .*\n(?: +.*\n)*\n")

func listUpNetworkInterfaces(ctx context.Context) ([]string, error) {
	output, err := testexec.CommandContext(ctx, "ifconfig").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get interface list")
	}
	var interfaces []string
	match := ifconfigRE.FindAllSubmatch(output, -1)
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
func DisableNetworkInterface(ctx context.Context, iface string) (CleanupCallback, error) {
	testing.ContextLogf(ctx, "Disabling network interface %q", iface)
	if err := disableNetworkInterface(ctx, iface); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Re-enabling network interface %q", iface)
		return enableNetworkInterface(ctx, iface)
	}, nil
}

// DisableNetworkInterfaces disables all network interfaces whose names match a
// regexp.
func DisableNetworkInterfaces(ctx context.Context, pattern *regexp.Regexp) (CleanupCallback, error) {
	return Nested(ctx, "disable network interface", func(s *Setup) error {
		upInterfaces, err := listUpNetworkInterfaces(ctx)
		if err != nil {
			return err
		}

		for _, iface := range upInterfaces {
			if !pattern.MatchString(iface) {
				continue
			}
			s.Add(DisableNetworkInterface(ctx, iface))
		}
		return nil
	})
}

// DisableWiFiInterfaces disables all WiFi adapters and returns a callback to
// re-enable them.
func DisableWiFiInterfaces(ctx context.Context) (CleanupCallback, error) {
	var wifiInterfacePattern = regexp.MustCompile(".*wlan\\d+")
	return DisableNetworkInterfaces(ctx, wifiInterfacePattern)
}
