// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// WiFi functions using shill.

package shill

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const wifiDefaultTimeout = 30 * time.Second

// WifiInterface polls the WiFi interface name with timeout.
// It returns "" with error if no (or more than one) WiFi interface is found.
func WifiInterface(ctx context.Context, m *Manager, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	wifiIfaces := func() ([]string, error) {
		_, props, err := m.DevicesByTechnology(ctx, TechnologyWifi)
		if err != nil {
			return nil, err
		}
		var ifaces []string
		for _, p := range props {
			if iface, err := p.GetString(shillconst.DevicePropertyInterface); err == nil {
				ifaces = append(ifaces, iface)
			}
		}
		return ifaces, nil
	}

	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	for {
		// If more than one WiFi interface is found, an error is raised.
		// If there's no WiFi interface, probe again when manager's "Devices" property is changed.
		if ifaces, err := wifiIfaces(); err != nil {
			return "", err
		} else if len(ifaces) > 1 {
			return "", errors.Errorf("more than one WiFi interface found: %q", ifaces)
		} else if len(ifaces) == 1 {
			return ifaces[0], nil
		}

		if _, err := pw.WaitAll(ctx, shillconst.ManagerPropertyDevices); err != nil {
			return "", err
		}
	}
}

// EnableWiFi enables or disables the WiFi network according to the given enable flag.
func EnableWiFi(ctx context.Context, enable bool) error {
	m, err := NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create shill Manager object")
	}
	// Make sure WiFi interface is available.
	if _, err := WifiInterface(ctx, m, 5*time.Second); err != nil {
		return errors.Wrap(err, "could not get system WiFi interface")
	}

	enabled, err := m.IsEnabled(ctx, TechnologyWifi)
	if err != nil {
		return errors.Wrap(err, "failed to get WiFi enabled state")
	}
	if enabled == enable {
		return nil
	}

	if enable {
		if err := m.EnableTechnology(ctx, TechnologyWifi); err != nil {
			errors.Wrap(err, "could not enable wifi via shill")
		}
	} else {
		if err := m.DisableTechnology(ctx, TechnologyWifi); err != nil {
			errors.Wrap(err, "could not disable wifi via shill")
		}
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		enabled, err := m.IsEnabled(ctx, TechnologyWifi)
		if err != nil {
			return errors.Wrap(err, "failed to get WiFi enabled state")
		}
		if enabled != enable {
			return errors.Errorf("failed to set WiFi enablement to %t", enable)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  wifiDefaultTimeout,
		Interval: 100 * time.Millisecond,
	})
}

// IsWiFiConnected returns true if any WiFi AP is connected.
func IsWiFiConnected(ctx context.Context) (bool, error) {
	m, err := NewManager(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to create shill Manager object")
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeWifi,
		shillconst.ServicePropertyIsConnected: true,
	}

	if _, err := m.FindMatchingService(ctx, props); err != nil {
		if err.Error() == shillconst.ErrorMatchingServiceNotFound {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to find the WiFi AP")
	}

	return true, nil
}

// IsWiFiAPConnected returns true if the given WiFi AP is connected.
func IsWiFiAPConnected(ctx context.Context, ssid string) (bool, error) {
	m, err := NewManager(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to create shill Manager object")
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID: strings.ToUpper(hex.EncodeToString([]byte(ssid))),
		shillconst.ServicePropertyVisible:     true, // only find visible APs.
	}

	service, err := m.FindMatchingService(ctx, props)
	if err != nil {
		if err.Error() == shillconst.ErrorMatchingServiceNotFound {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to find the WiFi AP")
	}

	connected, err := service.IsConnected(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get the WiFi service connected status")
	}
	return connected, nil
}

// ConnectWiFiAP connects to a given WiFi AP identified by SSID.
func ConnectWiFiAP(ctx context.Context, ssid, passphrase string) error {
	m, err := NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create shill Manager object")
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID: strings.ToUpper(hex.EncodeToString([]byte(ssid))),
		shillconst.ServicePropertyVisible:     true, // only find visible APs.
	}
	var service *Service
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		service, err = m.FindMatchingService(ctx, props)
		if err == nil {
			return nil
		}
		// Scan WiFi AP again if the expected AP is not found.
		if err := m.RequestScan(ctx, TechnologyWifi); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to request wifi active scan"))
		}
		return err
	}, &testing.PollOptions{
		Timeout:  wifiDefaultTimeout,
		Interval: 200 * time.Millisecond,
	}); err != nil {
		return errors.Wrap(err, "failed to find the WiFi AP")
	}

	connected, err := service.IsConnected(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the WiFi service connected status")
	}
	if connected {
		return nil
	}

	if err := service.SetProperty(ctx, shillconst.ServicePropertyPassphrase, passphrase); err != nil {
		return errors.Wrap(err, "failed to set service passphrase")
	}
	w, err := service.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer w.Close(ctx)

	if err := service.Connect(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to service")
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := service.IsConnected(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get WiFi connected status")
		}
		if !connected {
			return errors.New("wifi AP is not connected yet")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  wifiDefaultTimeout,
		Interval: 200 * time.Millisecond,
	})
}

// ForgetWiFiAP removes the WiFi AP from user profile.
func ForgetWiFiAP(ctx context.Context, ssid string) error {
	m, err := NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create shill Manager object")
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID: strings.ToUpper(hex.EncodeToString([]byte(ssid))),
	}

	service, err := m.FindMatchingService(ctx, props)
	if err != nil {
		return errors.Wrap(err, "cannot find the given WiFi AP service")
	}

	if err := service.Remove(ctx); err != nil {
		return errors.Wrap(err, "failed to remove the service")
	}
	return nil
}
