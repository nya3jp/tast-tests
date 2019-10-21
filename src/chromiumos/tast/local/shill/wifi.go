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

	// getIface returns the WiFi interface.
	getIface := func(ignoreGetDevError bool) (string, error) {
		devs, err := m.GetDevicesByTechnology(ctx, TechnologyWifi)
		if err != nil {
			if ignoreGetDevError {
				return "", nil
			}
			return "", err
		}
		var ifaces []string
		for _, dev := range devs {
			if iface, err := dev.Properties().GetString(DevicePropertyInterface); err == nil {
				ifaces = append(ifaces, iface)
			}
		}
		if len(ifaces) < 1 {
			return "", nil
		}
		if len(ifaces) > 1 {
			return "", errors.Errorf("more than one WiFi interfaces found: %q", ifaces)
		}
		return ifaces[0], nil
	}

	if iface, err := getIface(true); err != nil {
		return "", err
	} else if iface != "" {
		return iface, nil
	}

	// Failed getting WiFi interface, wait for shill's Device property change.
	pw, err := m.Properties().CreateWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	for {
		if err := pw.WaitAll(ctx, ManagerPropertyDevices); err != nil {
			return "", err
		}
		if iface, err := getIface(false); err != nil {
			return "", err
		} else if iface != "" {
			return iface, nil
		}
	}
}
