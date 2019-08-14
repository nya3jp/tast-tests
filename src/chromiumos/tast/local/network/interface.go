// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// Interfaces returns a list of network interfaces.
func Interfaces() ([]string, error) {
	files, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interfaces")
	}
	ifaces := make([]string, len(files))
	for i, file := range files {
		ifaces[i] = file.Name()
	}
	return ifaces, nil
}

// WifiInterface returns the first seen WiFi interface.
// It returns "" with error if no Wifi interface is found.
// The WiFi interface is obtained by querying WiFi device from shill device manager.
func WifiInterface(ctx context.Context) (string, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create shill manager proxy")
	}
	devPaths, err := m.GetDevicePaths(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to obtain paths of shill's devices")
	}
	for _, path := range devPaths {
		if dev, err := shill.NewDevice(ctx, path); err != nil {
			return "", err
		} else if devProps, err := dev.GetProperties(ctx); err != nil {
			return "", err
		} else if devProps[shill.DevicePropertyType].(string) == "wifi" {
			return devProps[shill.DevicePropertyName].(string), nil
		}
	}
	return "", errors.New("could not find a wireless interface")
}
