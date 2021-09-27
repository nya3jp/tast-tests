// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type wifiSecurityParams struct {
	wifiSecurity bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         APConnectDisconnect,
		Desc:         "Wi-Fi AP connect-disconnect with and without security mode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"wifissid", "wifipassword", "wifi.targetIteration"},
		Params: []testing.Param{{
			Name: "security_psk",
			Val:  wifiSecurityParams{wifiSecurity: true},
			Pre:  chrome.LoggedIn(),
		},
			{
				Name: "security_open",
				Val:  wifiSecurityParams{wifiSecurity: false},
				Pre:  chrome.LoggedIn(),
			},
		},
	})
}

func APConnectDisconnect(ctx context.Context, s *testing.State) {
	wifiOpt := s.Param().(wifiSecurityParams)
	// Wi-Fi's SSID and password as command-line argument.
	ssid := s.RequiredVar("wifissid")
	wifiPwd := s.RequiredVar("wifipassword")
	defaultIter := 10
	newCounter, ok := s.Var("wifi.targetIteration")
	if ok {
		defaultIter, _ = strconv.Atoi(newCounter)
	}
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	expectProps := map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyName:          ssid,
		shillconst.ServicePropertySecurityClass: shillconst.SecurityNone,
	}
	if wifiOpt.wifiSecurity {
		expectProps = map[string]interface{}{
			shillconst.ServicePropertyType:          shillconst.TypeWifi,
			shillconst.ServicePropertyName:          ssid,
			shillconst.ServicePropertySecurityClass: shillconst.SecurityPSK,
		}
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
	for i := 1; i <= defaultIter; i++ {
		s.Logf("Iteration %d of %d", i, defaultIter)
		service, err := manager.FindMatchingService(ctx, expectProps)
		if err != nil {
			s.Fatal("Failed to find matching services: ", err)
		}
		if wifiOpt.wifiSecurity {
			if err := service.SetProperty(ctx, shillconst.ServicePropertyPassphrase, wifiPwd); err != nil {
				s.Fatal("Failed to set service passphrase: ", err)
			}
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Scan WiFi AP again if the expected AP is not found.
			if err := manager.RequestScan(ctx, shill.TechnologyWifi); err != nil {
				return errors.Wrap(err, "failed to request wifi active scan")
			}
			return nil
		}, &testing.PollOptions{
			Timeout: 5 * time.Second,
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
			Timeout: 5 * time.Second,
		}); err != nil {
			s.Fatalf("Failed to connect to %s Wi-Fi SSID: %v", ssid, err)
		}
		s.Log("Disconnecting AP")
		if err := service.Disconnect(ctx); err != nil {
			s.Fatal("Failed to remove the service: ", err)
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
}
