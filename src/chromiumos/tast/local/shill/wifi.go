// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// WiFi functions using shill.

package shill

import (
	"context"
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
		devs, err := m.GetDevicesByTechnology(ctx, TechnologyWifi)
		if err != nil {
			return nil, err
		}
		var ifaces []string
		for _, dev := range devs {
			if iface, err := dev.Properties().GetString(DevicePropertyInterface); err == nil {
				ifaces = append(ifaces, iface)
			}
		}
		return ifaces, nil
	}

	pw, err := m.Properties().CreateWatcher(ctx)
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

<<<<<<< HEAD   (3a4a84 chrome: Ignore errors of rpcc.Conn.Close.)
		if devType, err := dev.GetStringProp(DevicePropertyType); err != nil {
			return "", err
		} else if devType != "wifi" {
			continue
		}

		devIface, err := dev.GetStringProp(DevicePropertyInterface)
		if err != nil {
=======
		if err := pw.WaitAll(ctx, ManagerPropertyDevices); err != nil {
>>>>>>> CHANGE (bba4c1 Tast: Poll the WiFi interface name until timeout.)
			return "", err
		}
	}
}
