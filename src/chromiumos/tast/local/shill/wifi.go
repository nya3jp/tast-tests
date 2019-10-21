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

// getWifiInterface gets the WiFi interface name from shill devices.
// It returns "" if no WiFi interface is found.
// It returns error if more than one WiFi interface is found.
func (m *Manager) getWifiInterface(ctx context.Context) (string, error) {
	var wifis []string
	// Refresh properties first.
	m.GetProperties(ctx)
	devPaths, err := m.GetDevices(ctx)
	if err != nil {
		return "", err
	}

	for _, path := range devPaths {
		dev, err := NewDevice(ctx, path)
		if err != nil {
			return "", err
		}

		if devType, err := dev.Properties().GetString(DevicePropertyType); err != nil {
			return "", err
		} else if devType != "wifi" {
			continue
		}

		devIface, err := dev.Properties().GetString(DevicePropertyInterface)
		if err != nil {
			return "", err
		}
		wifis = append(wifis, devIface)
	}

	if len(wifis) > 1 {
		return "", errors.Errorf("expect only one WiFi interface, found: %q", wifis)
	} else if len(wifis) < 1 {
		return "", nil
	}

	return wifis[0], nil
}

// GetWifiInterface polls the WiFi interface name with timeout.
// It returns "" with error if no (or more than one) WiFi interface is found.
func GetWifiInterface(ctx context.Context, timeout time.Duration) (string, error) {
	ctx, cancel := ctxutil.OptionalTimeout(ctx, timeout)
	defer cancel()

	m, err := NewManager(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create shill manager proxy")
	}

	if wifi, err := m.getWifiInterface(ctx); err == nil && wifi != "" {
		return wifi, nil
	}

	// Failed getting WiFi interface, wait for shill's Device property change.
	pw, err := m.Properties().CreateWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer func() {
		pw.Close(ctx)
	}()

	for {
		if err := pw.WaitAll(ctx, "Devices"); err != nil {
			return "", err
		}

		if wifi, err := m.getWifiInterface(ctx); err != nil {
			return "", errors.Wrap(err, "failed to get WiFi interface from shill")
		} else if wifi != "" {
			return wifi, nil
		}
	}
}
