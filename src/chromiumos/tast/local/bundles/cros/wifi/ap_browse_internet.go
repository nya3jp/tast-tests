// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APBrowseInternet,
		Desc:         "Wi-Fi AP connect and browse internet",
		Contacts:     []string{"pathan.jilani@intel.com", "cros-network-health@google.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"wifissid", "wifipassword"},
	})
}

func APBrowseInternet(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatalf("Failed to start chrome: %s", err)
	}

	ssid := s.RequiredVar("wifissid")
	wifiPwd := s.RequiredVar("wifipassword")
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	expectProps := map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyName:          ssid,
		shillconst.ServicePropertySecurityClass: shillconst.SecurityPSK,
	}

	enableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet)
	if err != nil {
		s.Fatal("Unable to disable ethernet: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
	defer cancel()
	defer enableFunc(cleanupCtx)

	if err := enableWifi(ctx, manager); err != nil {
		s.Fatal("Failed to enable wifi: ", err)
	}

	service, err := manager.FindMatchingService(ctx, expectProps)
	if err != nil {
		s.Fatal("Failed to find matching services: ", err)
	}

	if err := setWifiProperties(ctx, manager, service, wifiPwd); err != nil {
		s.Fatal("Failed to set wifi properties: ", err)
	}

	s.Log("Connecting AP")
	if err := service.Connect(ctx); err != nil {
		s.Fatal(err, "Failed to connect to service")
	}

	if err := wifiConnected(ctx, service); err != nil {
		s.Fatal("Failed as wifi is disconnected: ", err)
	}

	if err := browseInternet(ctx, cr); err != nil {
		s.Fatal("Failed to browse internet over wifi: ", err)
	}

	s.Log("Disconnecting AP")
	if err := service.Disconnect(ctx); err != nil {
		s.Fatal("Failed to remove the service: ", err)
	}

	if err := wifiConnected(ctx, service); err == nil {
		s.Fatal("Failed as wifi is still connected: ", err)
	}
}

func enableWifi(ctx context.Context, manager *shill.Manager) error {
	if err := manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
		return errors.Wrap(err, "failed to enable WiFi")
	}

	if enabled, err := manager.IsEnabled(ctx, shill.TechnologyWifi); err != nil {
		return errors.Wrap(err, "failed to get WiFi enabled state")
	} else if !enabled {
		return errors.New("failed to enable wifi")
	}
	testing.ContextLog(ctx, "Wi-Fi is enabled")
	return nil
}

func wifiConnected(ctx context.Context, service *shill.Service) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := service.IsConnected(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get WiFi connected state")
		}
		if !connected {
			return errors.New("wi-fi is disconnected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to connect to Wi-Fi SSID")
	}
	return nil
}

func setWifiProperties(ctx context.Context, manager *shill.Manager, service *shill.Service, wifiPassword string) error {
	if err := service.SetProperty(ctx, shillconst.ServicePropertyPassphrase, wifiPassword); err != nil {
		return errors.Wrap(err, "failed to set service passphrase")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := manager.RequestScan(ctx, shill.TechnologyWifi); err != nil {
			return errors.Wrap(err, "failed to request wifi active scan")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 20 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to find the WiFi AP")
	}
	return nil
}

func browseInternet(ctx context.Context, cr *chrome.Chrome) error {
	var browseweb = "https://www.google.com/"
	conn, err := cr.NewConn(ctx, browseweb)
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}
	defer conn.Close()
	var actual string
	expected := "Google Search"

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, `document.querySelector('[aria-label="Google Search"]').value`, &actual); err != nil {
			return errors.Wrap(err, "failed getting page content")
		}
		if actual != expected {
			return errors.Errorf("unexpected page content: got %s; expecting %s", actual, expected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 5 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to open required url page")
	}
	return nil
}
