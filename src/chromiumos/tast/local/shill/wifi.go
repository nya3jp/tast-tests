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

// WifiInterfaces retrieives all WiFi interfaces' names in m.
// The caller should be aware of that its result is a snapshot.
// For environment that expectes WiFi interfaces to change, caller is responsible to take
// multiple snapshots with PropertiesWatcher to make sure the expected behavior is observed.
// Refer to WifiInterface() for detail.
func WifiInterfaces(ctx context.Context, m *Manager) ([]string, error) {
	_, props, err := m.DevicesByTechnology(ctx, TechnologyWifi)
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

// WifiInterface polls the WiFi interface name with timeout.
// It returns "" with error if no (or more than one) WiFi interface is found.
func WifiInterface(ctx context.Context, m *Manager, timeout time.Duration) (string, error) {
	ctx, cancel := ctxutil.OptionalTimeout(ctx, timeout)
	defer cancel()

	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	for {
		// If more than one WiFi interface is found, an error is raised.
		// If there's no WiFi interface, probe again when manager's "Devices" property is changed.
		if ifaces, err := WifiInterfaces(ctx, m); err != nil {
			return "", err
		} else if len(ifaces) > 1 {
			return "", errors.Errorf("more than one WiFi interface found: %q", ifaces)
		} else if len(ifaces) == 1 {
			return ifaces[0], nil
		}

		if _, err := pw.WaitAll(ctx, ManagerPropertyDevices); err != nil {
			return "", err
		}
	}
}
