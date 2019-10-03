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
// Bbtained by querying WiFi device from shill.
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
		if dev, err := NewDevice(ctx, path); err != nil {
			return "", err
		} else if props, err := dev.GetProps(ctx); err != nil {
			return "", err
		} else if props[DevicePropertyType].(string) == "wifi" {
			wifis = append(wifis, props[DevicePropertyName].(string))
		}
	}
	if len(wifis) == 1 {
		return wifis[0], nil
	}
	if len(wifis) == 0 {
		return "", errors.New("could not find a WiFi interface")
	}
	return "", errors.Errorf("found more than one WiFi interfaces: %s", wifis)
}
