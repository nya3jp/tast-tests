// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssistantBluetoothWifiQueries,
		Desc:         "Tests toggling Bluetooth and WiFi using Assistant queries",
		Contacts:     []string{"kyleshima@chromium.org", "bhansknecht@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// AssistantBluetoothWifiQueries tests that Assistant queries can be used to toggle Bluetooth and WiFi on/off
func AssistantBluetoothWifiQueries(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Starts Assistant service.
	if err := assistant.Enable(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	s.Log("Waiting for Assistant to be ready to answer queries")
	if err := assistant.WaitForServiceReady(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Libassistant to become ready: ", err)
	}

	// Open the Settings window, where we can verify Bluetooth/Wifi status
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}

	// Turn settings on, off, and on again to ensure they can be enabled and disabled, regardless of starting state
	statuses := []bool{true, false, true}
	var onOff string
	for _, status := range statuses {
		if status {
			onOff = "on"
		} else {
			onOff = "off"
		}

		for _, setting := range []string{"Bluetooth", "WiFi"} {
			s.Logf("Turning %v %v", setting, onOff)
			// Queries sometimes don't go through when running several queries in the test, so poll to ensure the queries go through.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				_, err := assistant.SendTextQuery(ctx, tconn, fmt.Sprintf("turn %v %v", setting, onOff))
				return err
			}, nil); err != nil {
				s.Fatalf("Failed to get Assistant %v query response: %v", setting, err)
			}
		}

		// Check results in the Settings app. The buttons don't update immediately, so we'll need to poll their statuses.
		// The "aria-pressed" htmlAttribute of the toggle buttons can be used to check the on/off status
		s.Log("Checking bluetooth toggle button status")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			params := ui.FindParams{Name: "Bluetooth enable", Role: ui.RoleTypeToggleButton}
			bluetoothToggle, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
			if err != nil {
				s.Fatal("Failed to get Bluetooth toggle button from UI: ", err)
			}
			defer bluetoothToggle.Release(ctx)

			if bluetoothToggle.HTMLAttributes["aria-pressed"] != strconv.FormatBool(status) {
				return errors.Errorf("bluetooth not toggled yet, aria-pressed is %v", bluetoothToggle.HTMLAttributes["aria-pressed"])
			}
			return nil
		}, nil); err != nil {
			s.Fatalf("Bluetooth was not turned %v by the Assistant query: %v", onOff, err)
		}

		s.Log("Checking wifi toggle button status")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			params := ui.FindParams{Name: "Wi-Fi enable", Role: ui.RoleTypeToggleButton}
			wifiToggle, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
			if err != nil {
				s.Fatal("Failed to get WiFi toggle button from UI: ", err)
			}
			defer wifiToggle.Release(ctx)

			if wifiToggle.HTMLAttributes["aria-pressed"] != strconv.FormatBool(status) {
				return errors.Errorf("wifi not toggled yet, aria-pressed is %v", wifiToggle.HTMLAttributes["aria-pressed"])
			}
			return nil
		}, nil); err != nil {
			s.Fatalf("WiFi was not turned %v by the Assistant query: %v", onOff, err)
		}
	}
}
