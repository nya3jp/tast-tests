// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// WiFi functions using shill.

package shill

import (
	"context"
	"fmt"
	"net"
	"sort"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
)

// interfaceNames returns a list of network interface names.
func interfaceNames() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(ifaces))
	for i := range ifaces {
		names[i] = ifaces[i].Name
	}
	sort.Strings(names)
	return names, nil
}

// GetWifiInterface polls the WiFi interface name with timeout.
// It returns "" with error if no (or more than one) WiFi interface is found.
func GetWifiInterface(ctx context.Context, m *Manager, timeout time.Duration) (string, error) {
	ctx, cancel := ctxutil.OptionalTimeout(ctx, timeout)
	defer cancel()

	getWifiIfaces := func() ([]string, error) {
		_, props, err := m.GetDevicesByTechnology(ctx, TechnologyWifi)
		if err != nil {
			return nil, err
		}
		var ifaces []string
		for _, p := range props {
			if iface, err := p.GetString(DevicePropertyInterface); err == nil {
				ifaces = append(ifaces, iface)
			}
		}
		return ifaces, nil
	}

	// netIfaceStr composes a string of network interfaces.
	netIfaceStr := func() string {
		ifaces, err := interfaceNames()
		if err != nil {
			return "unable to get network interfaces: " + err.Error()
		}
		return fmt.Sprintf("network interfaces: %q", ifaces)
	}

	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "shill: failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	for {
		// If more than one WiFi interface is found, an error is raised.
		// If there's no WiFi interface, probe again when manager's "Devices" property is changed.
		if ifaces, err := getWifiIfaces(); err != nil {
			return "", errors.Errorf("unable to query device property from shill (%s): %q", netIfaceStr(), err)
		} else if len(ifaces) > 1 {
			return "", errors.Errorf("more than one WiFi interface found (%s): %q", netIfaceStr(), ifaces)
		} else if len(ifaces) == 1 {
			return ifaces[0], nil
		}

		if _, err := pw.WaitAll(ctx, ManagerPropertyDevices); err != nil {
			return "", errors.Wrapf(err, "shill: failed to wait for Devices update (%s)", netIfaceStr())
		}
	}
}
