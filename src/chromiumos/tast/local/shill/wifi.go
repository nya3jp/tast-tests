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

	if wifis, err := m.GetInterfaces(ctx, TechnologyWifi); err == nil && len(wifis) == 1 {
		return wifis[0], nil
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

		if wifis, err := m.GetInterfaces(ctx, TechnologyWifi); err != nil {
			return "", err
		} else if len(wifis) == 1 {
			return wifis[0], nil
		} else if len(wifis) > 1 {
			return "", errors.Errorf("more than one WiFi interfaces found: %q", wifis)
		}
	}
}
