// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wlan

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// EnableWiFi enables WiFi technology.
func EnableWiFi(ctx context.Context, manager *shill.Manager) error {
	if err := manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
		return errors.Wrap(err, "failed to enable WiFi")
	}

	if enabled, err := manager.IsEnabled(ctx, shill.TechnologyWifi); err != nil {
		return errors.Wrap(err, "failed to get WiFi enabled state")
	} else if !enabled {
		return errors.New("failed to enable WiFi")
	}
	testing.ContextLog(ctx, "WiFi is enabled")
	return nil
}

// WiFiConnected check whether WiFi is connected or not.
func WiFiConnected(ctx context.Context, service *shill.Service) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := service.IsConnected(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get WiFi connected state")
		}
		if !connected {
			return errors.New("WiFi is disconnected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to connect to WiFi SSID")
	}
	return nil
}

// SetWiFiProperties will set WiFi properties.
func SetWiFiProperties(ctx context.Context, manager *shill.Manager, service *shill.Service, wifiPassword string) error {
	if err := service.SetProperty(ctx, shillconst.ServicePropertyPassphrase, wifiPassword); err != nil {
		return errors.Wrap(err, "failed to set service passphrase")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := manager.RequestScan(ctx, shill.TechnologyWifi); err != nil {
			return errors.Wrap(err, "failed to request WiFi active scan")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 20 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to find the WiFi AP")
	}
	return nil
}
