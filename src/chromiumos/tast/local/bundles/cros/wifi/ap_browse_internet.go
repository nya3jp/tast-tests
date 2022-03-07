// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wifi/wlan"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APBrowseInternet,
		Desc:         "WiFi AP connect and browse internet",
		Contacts:     []string{"pathan.jilani@intel.com", "cros-network-health@google.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"wifissid", "wifipassword"},
	})
}

func APBrowseInternet(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	ssid := s.RequiredVar("wifissid")
	wifiPwd := s.RequiredVar("wifipassword")
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	expectProps := map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyName:          ssid,
		shillconst.ServicePropertySecurityClass: shillconst.SecurityPSK,
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
	defer cancel()

	enableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet)
	if err != nil {
		s.Fatal("Unable to disable ethernet: ", err)
	}
	defer enableFunc(cleanupCtx)

	if err := wlan.EnableWiFi(ctx, manager); err != nil {
		s.Fatal("Failed to enable WiFi: ", err)
	}

	service, err := manager.FindMatchingService(ctx, expectProps)
	if err != nil {
		s.Fatal("Failed to find matching services: ", err)
	}

	if err := wlan.SetWiFiProperties(ctx, manager, service, wifiPwd); err != nil {
		s.Fatal("Failed to set WiFi properties: ", err)
	}

	s.Log("Connecting AP")
	if err := service.Connect(ctx); err != nil {
		s.Fatal(err, "Failed to connect to service")
	}

	if err := wlan.WiFiConnected(ctx, service); err != nil {
		s.Fatal("Failed as WiFi is disconnected: ", err)
	}

	if err := browseInternet(ctx, cr); err != nil {
		s.Fatal("Failed to browse internet over WiFi: ", err)
	}

	s.Log("Disconnecting AP")
	if err := service.Disconnect(ctx); err != nil {
		s.Fatal("Failed to remove the service: ", err)
	}

	if err := wlan.WiFiConnected(ctx, service); err == nil {
		s.Fatal("Failed as WiFi is still connected: ", err)
	}
}

// browseInternet will browse Google webpage.
func browseInternet(ctx context.Context, cr *chrome.Chrome) error {
	var browseweb = "https://www.google.com/"
	conn, err := cr.NewConn(ctx, browseweb)
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}
	defer conn.Close()
	return nil
}
