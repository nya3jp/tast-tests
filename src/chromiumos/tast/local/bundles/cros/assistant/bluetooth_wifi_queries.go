// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BluetoothWifiQueries,
		Desc:         "Tests toggling Bluetooth and WiFi using Assistant queries",
		Contacts:     []string{"kyleshima@chromium.org", "bhansknecht@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// BluetoothWifiQueries tests that Assistant queries can be used to toggle Bluetooth and WiFi on/off
func BluetoothWifiQueries(ctx context.Context, s *testing.State) {
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

		// Check actual statuses of Bluetooth and Wifi
		s.Log("Checking Bluetooth status using dbus")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if enabled, err := bluetoothEnabled(ctx); enabled != status {
				return errors.Wrapf(err, "bluetooth enabled %v", enabled)
			}
			return nil
		}, nil); err != nil {
			s.Fatal("Failed checking bluetooth status via dbus: ", err)
		}

		s.Log("Checking wifi status using ifconfig")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			netInts, err := networkInterfaces(ctx)
			if err != nil {
				return testing.PollBreak(err)
			}
			wifiOn := false
			for _, n := range netInts {
				if n == "wlan0" {
					wifiOn = true
				}
			}
			if wifiOn == status {
				return nil
			}
			return errors.Errorf("wifi status not changed; active interfaces are %v", strings.Join(netInts, ", "))
		}, nil); err != nil {
			s.Fatal("Wifi status not changed by the Assistant: ", err)
		}

		// Check if buttons in the Settings app UI updated to match the actual status.
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
			s.Fatalf("WiFi toggle button was not turned %v by the Assistant query: %v", onOff, err)
		}

		// Check Bluetooth ubertray button as well.
		// Skip checking the Wifi one, since it will not show Wifi status if the DUT has a wired connection.
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

// networkInterfaces gets the list of active network interfaces from ifconfig
func networkInterfaces(ctx context.Context) ([]string, error) {
	var ifRegex = regexp.MustCompile(`^\w+`)
	out, err := testexec.CommandContext(ctx, "ifconfig").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get network interfaces")
	}
	lines := strings.Split(string(out), "\n")

	var res []string
	for _, l := range lines {
		match := ifRegex.FindString(l)
		if match != "" {
			res = append(res, match)
		}
	}
	return res, nil
}
