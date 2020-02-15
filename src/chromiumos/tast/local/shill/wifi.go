// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// WiFi functions using shill.

package shill

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
)

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

	// netDevices obtains net devices under /sys/class/net
	netDevices := func() ([]string, error) {
		files, err := ioutil.ReadDir("/sys/class/net")
		if err != nil {
			return nil, err
		}
		var ifaces []string
		for _, f := range files {
			ifaces = append(ifaces, f.Name())
		}
		return ifaces, nil
	}

	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	for {
		// If more than one WiFi interface is found, an error is raised.
		// If there's no WiFi interface, probe again when manager's "Devices" property is changed.
		if ifaces, err := getWifiIfaces(); err != nil {
			return "", err
		} else if len(ifaces) > 1 {
			return "", errors.Errorf("more than one WiFi interface found: %q", ifaces)
		} else if len(ifaces) == 1 {
			return ifaces[0], nil
		}

		if _, err := pw.WaitAll(ctx, ManagerPropertyDevices); err != nil {
			var sysClassNet string
			if netDevs, err := netDevices(); err != nil {
				sysClassNet = "/sys/class/net inaccessible: " + err.Error()
			} else {
				sysClassNet = fmt.Sprintf("/sys/class/net: %q", netDevs)
			}
			return "", errors.Wrapf(err, "shill: timeout waiting Devices update (%s)", sysClassNet)
		}
	}
}
