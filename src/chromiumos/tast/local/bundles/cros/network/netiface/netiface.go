// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package netiface provides helpers accessing network interfaces.
package netiface

import (
	"context"
	"fmt"
	"net"
	"sort"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// InterfaceNames returns a list of network interface names.
func InterfaceNames() ([]string, error) {
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

// WifiInterface polls the WiFi interface name with timeout.
// It returns "" with error if no (or more than one) WiFi interface is found.
func WifiInterface(ctx context.Context, m *shill.Manager, timeout time.Duration) (string, error) {
	ctx, cancel := ctxutil.OptionalTimeout(ctx, timeout)
	defer cancel()

	queryShill := func() ([]string, error) {
		_, props, err := m.GetDevicesByTechnology(ctx, shill.TechnologyWifi)
		if err != nil {
			return nil, err
		}
		var ifaces []string
		for _, p := range props {
			if iface, err := p.GetString(shill.DevicePropertyInterface); err == nil {
				ifaces = append(ifaces, iface)
			}
		}
		return ifaces, nil
	}

	// sysClassNet composes a string of devices under /sys/class/net.
	sysClassNet := func() string {
		netDevs, err := InterfaceNames()
		if err != nil {
			return "/sys/class/net inaccessible: " + err.Error()
		}
		return fmt.Sprintf("/sys/class/net: %q", netDevs)
	}

	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "shill: failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	for {
		// If more than one WiFi interface is found, an error is raised.
		// If there's no WiFi interface, probe again when manager's "Devices" property is changed.
		if ifaces, err := queryShill(); err != nil {
			return "", errors.Errorf("unable to query device property from shill (%s): %q", sysClassNet(), err)
		} else if len(ifaces) > 1 {
			return "", errors.Errorf("more than one WiFi interface found (%s): %q", sysClassNet(), ifaces)
		} else if len(ifaces) == 1 {
			return ifaces[0], nil
		}

		if _, err := pw.WaitAll(ctx, shill.ManagerPropertyDevices); err != nil {
			return "", errors.Wrapf(err, "timeout waiting Devices update from shill (%s)", sysClassNet())
		}
	}
}
