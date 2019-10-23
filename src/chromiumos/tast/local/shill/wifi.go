// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// WiFi functions using shill.

package shill

import (
	"context"

	"chromiumos/tast/errors"
)

// GetWifiInterface returns the WiFi interface name.
// It returns "" with error if no (or more than one) WiFi interface is found.
// Obtained by querying WiFi device from shill.
func GetWifiInterface(ctx context.Context) (string, error) {
	m, err := NewManager(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create shill manager proxy")
	}
	devPaths, err := m.GetDevices(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to obtain paths of shill's devices")
	}
	var wifis []string
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
	if len(wifis) != 1 {
		return "", errors.Errorf("expect only one WiFi interface, found: %q", wifis)
	}
	return wifis[0], nil
}
