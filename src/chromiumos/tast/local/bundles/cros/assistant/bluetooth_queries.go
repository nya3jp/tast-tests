// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BluetoothQueries,
		Desc:         "Tests toggling Bluetooth using Assistant queries",
		Contacts:     []string{"kyleshima@chromium.org", "bhansknecht@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// BluetoothQueries tests that Assistant queries can be used to toggle Bluetooth on/off
func BluetoothQueries(ctx context.Context, s *testing.State) {
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

		s.Log("Turning bluetooth ", onOff)
		// Queries sometimes don't go through when running several queries in the test, so poll to ensure the queries go through.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			_, err := assistant.SendTextQuery(ctx, tconn, fmt.Sprintf("turn bluetooth %v", onOff))
			return err
		}, nil); err != nil {
			s.Fatal("Failed to get Assistant bluetooth query response: ", err)
		}

		s.Log("Checking Bluetooth status using dbus")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if enabled, err := bluetoothEnabled(ctx); enabled != status {
				return errors.Wrapf(err, "bluetooth enabled %v", enabled)
			}
			return nil
		}, nil); err != nil {
			s.Fatal("Failed checking bluetooth status via dbus: ", err)
		}

		// Check if button in the Settings app UI updated to match the actual status.
		// The buttons don't update immediately, so we'll need to poll their statuses.
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
			s.Fatalf("Bluetooth toggle button was not turned %v by the Assistant query: %v", onOff, err)
		}

		// Check Bluetooth ubertray button as well.
		s.Log("Checking bluetooth ubertray pod button status")
		params := ui.FindParams{
			ClassName: "ash/StatusAreaWidgetDelegate",
		}
		statusArea, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
		if err != nil {
			s.Fatal("Failed to open ubertray: ", err)
		}
		defer statusArea.Release(ctx)

		if err := statusArea.LeftClick(ctx); err != nil {
			s.Fatal("Failed to open ubertray: ", err)
		}

		btnName := fmt.Sprintf("Toggle Bluetooth. Bluetooth is %v", onOff)
		if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: btnName}, 10*time.Second); err != nil {
			s.Fatal("Bluetooth button not toggled by Assistant: ", err)
		}
	}
}

// bluetoothEnabled checks if the bluetooth adapter is enabled using dbus
func bluetoothEnabled(ctx context.Context) (bool, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get bluetooth adapters")
	}
	if len(adapters) != 1 {
		return false, errors.Errorf("unexpected Bluetooth adapters count; got %d, want 1", len(adapters))
	}
	adapter := adapters[0]
	return adapter.Powered(ctx)
}
