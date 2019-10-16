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
	devPaths, err := m.GetDevices(ctx)
	if err != nil {
		return "", err
	}

	for _, path := range devPaths {
		dev, err := NewDevice(ctx, path)
		if err != nil {
			return "", err
		}

		if devType, err := dev.GetStringProp(DevicePropertyType); err != nil {
			return "", err
		} else if devType != "wifi" {
			continue
		}
		devIface, err := dev.GetStringProp(DevicePropertyInterface)
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
	w, err := m.CreatePropertyChangedWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create PropertyChangeWatcher")
	}
	defer func() {
		w.Close(ctx)
	}()

	for {
		if _, _, err := w.WaitFor(ctx, []string{"Devices"}); err != nil {
			return "", errors.Wrap(err, "failed to wait for shill manager's PropertyChange")
		}

		if wifi, err := m.getWifiInterface(ctx); err != nil {
			return "", errors.Wrap(err, "failed to get WiFi interface from shill")
		} else if wifi != "" {
			return wifi, nil
		}
	}
}
