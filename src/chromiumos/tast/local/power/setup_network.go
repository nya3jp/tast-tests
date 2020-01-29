// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// ifconfigRe parses one adapter from the output of ifconfig.
var ifconfigRe = regexp.MustCompile("([^:]+): .*\n(?: +.*\n)*\n")

// parseIfconfigOutput gets a list of adapters from ifconfig output.
func parseIfconfigOutput(output []byte) ([]string, error) {
	var interfaces []string
	match := ifconfigRe.FindAllSubmatch(output, -1)
	if match == nil {
		return interfaces, errors.Errorf("unable to parse interface list from %q", output)
	}
	for _, submatch := range match {
		interfaces = append(interfaces, string(submatch[1]))
	}
	return interfaces, nil
}

// disableNetworkInterfaces is an Action that disables all network
// interfaces that match a regexp.
type disableNetworkInterfaces struct {
	ctx      context.Context
	pattern  *regexp.Regexp
	reenable []string
}

// Setup disables all enabled network interfaces that match a regexp.
func (a *disableNetworkInterfaces) Setup() error {
	output, err := testexec.CommandContext(a.ctx, "ifconfig").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "unable to get interface list")
	}
	upInterfaces, err := parseIfconfigOutput(output)
	if err != nil {
		return err
	}

	for _, iface := range upInterfaces {
		if !a.pattern.MatchString(iface) {
			continue
		}
		if err := testexec.CommandContext(a.ctx, "ifconfig", iface, "down").Run(testexec.DumpLogOnError); err != nil {
			a.Cleanup()
			return errors.Wrapf(err, "unable to disable network interface %q", iface)
		}
		a.reenable = append(a.reenable, iface)
	}
	return nil
}

// Cleanup reenables all disabled network interfaces.
func (a *disableNetworkInterfaces) Cleanup() error {
	var result error
	for _, iface := range a.reenable {
		if err := testexec.CommandContext(a.ctx, "ifconfig", iface, "up").Run(testexec.DumpLogOnError); err != nil {
			result = err
		}
	}
	return result
}

// DisableNetworkInterfaces creates an Action that disables all
// network interfaces that match a Regexp.
func DisableNetworkInterfaces(ctx context.Context, pattern *regexp.Regexp) Action {
	return &disableNetworkInterfaces{
		ctx:      ctx,
		pattern:  pattern,
		reenable: []string{},
	}
}

// DisableWiFiInterfaces creates an Action that disables all Wifi interfaces.
func DisableWiFiInterfaces(ctx context.Context) Action {
	var wifiInterfaceRe = regexp.MustCompile(".*wlan\\d+")
	return DisableNetworkInterfaces(ctx, wifiInterfaceRe)
}
