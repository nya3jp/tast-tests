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
		Func:         WifiAPBrowseInternet,
		Desc:         "Wi-Fi AP connect and browse internet",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"wifissid", "wifipassword"},
	})
}

func WifiAPBrowseInternet(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatalf("Failed to start chrome: %s", err)
	}
	const browseweb = "https://www.google.com/"
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
	if enableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Unable to disable Ethernet: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
		ctx = newCtx
	}
	if err := manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Failed to enable WiFi: ", err)
	}
	enabled, err := manager.IsEnabled(ctx, shill.TechnologyWifi)
	if err != nil {
		s.Fatal("Failed to get WiFi enabled state: ", err)
	}
	if !enabled {
		s.Fatal("Failed to enable wifi: ", err)
	}
	s.Log("Wi-Fi is enabled")
	service, err := manager.FindMatchingService(ctx, expectProps)
	if err != nil {
		s.Fatal("Failed to find matching services: ", err)
	}
	if err := service.SetProperty(ctx, shillconst.ServicePropertyPassphrase, wifiPwd); err != nil {
		s.Fatal("Failed to set service passphrase: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := manager.RequestScan(ctx, shill.TechnologyWifi); err != nil {
			return errors.Wrap(err, "failed to request wifi active scan")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 20 * time.Second,
	}); err != nil {
		s.Fatal("Failed to find the WiFi AP: ", err)
	}
	s.Log("Connecting AP")
	if err := service.Connect(ctx); err != nil {
		s.Fatal(err, "Failed to connect to service")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := service.IsConnected(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get WiFi connected state")
		}
		if !connected {
			return errors.New("Wi-Fi is disconnected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		s.Fatalf("Failed to connect to %s Wi-Fi SSID: %v", ssid, err)
	}
	conn, err := cr.NewConn(ctx, browseweb)
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()
	var actual string
	expected := "Google Search"
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, `document.querySelector('[aria-label="Google Search"]').value`, &actual); err != nil {
			return errors.Wrap(err, "failed getting page content")
		}
		if actual != expected {
			return errors.Errorf("Unexpected page content: got %s; expecting %s", actual, expected)
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 5 * time.Second,
	}); err != nil {
		s.Fatalf("Failed to open required url page: ", err)
	}
	s.Log("Disconnecting AP")
	if err := service.Disconnect(ctx); err != nil {
		s.Fatal("failed to remove the service: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := service.IsConnected(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get WiFi connected state")
		}
		if connected {
			return errors.New("Wi-Fi is connected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 5 * time.Second,
	}); err != nil {
		s.Fatalf("Failed to disconnect from %s Wi-Fi SSID: %v", ssid, err)
	}
}
