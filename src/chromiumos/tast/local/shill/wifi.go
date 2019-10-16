// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// WiFi functions using shill.

package shill

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// getWifiInterface gets the WiFi interface name from the given shill devices.
// It returns "" with error if no (or more than one) WiFi interface is found.
func getWifiInterface(ctx context.Context, devPaths []dbus.ObjectPath) (string, error) {
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

// GetWifiInterface polls the WiFi interface name with timeout.
// It returns "" with error if no (or more than one) WiFi interface is found.
// Obtained by polling WiFi device from shill.
func GetWifiInterface(ctx context.Context, timeout time.Duration) (string, error) {
	ctx, cancel := ctxutil.OptionalTimeout(ctx, timeout)
	defer cancel()

	m, err := NewManager(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create shill manager proxy")
	}

	var wifi string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devPaths, err := m.PollDevices(ctx, 0)
		if err != nil {
			return err
		}
		if len(devPaths) < 1 {
			return errors.New("shill lists no devices")
		}

		wifi, err = getWifiInterface(ctx, devPaths)
		if err != nil {
			return err
		}
		if wifi == "" {
			return errors.New("no WiFi device found")
		}
		return nil
	}, nil); err != nil {
		return "", err
	}
	return wifi, nil
}
