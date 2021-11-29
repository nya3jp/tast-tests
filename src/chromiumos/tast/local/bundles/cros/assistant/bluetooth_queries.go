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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BluetoothQueries,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests toggling Bluetooth using Assistant queries",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "assistive-eng@google.com", "meilinw@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          assistant.VerboseLoggingEnabled(),
	})
}

// BluetoothQueries tests that Assistant queries can be used to toggle Bluetooth on/off
func BluetoothQueries(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable the Assistant and wait for the ready signal.
	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer assistant.Disable(ctx, tconn)

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
		// assistant.SendTextQuery sometimes times out after the assistant UI is closed,
		// so poll to ensure the queries go through.
		// TODO(crbug/1080363): remove polling.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			_, err := assistant.SendTextQuery(ctx, tconn, fmt.Sprintf("turn bluetooth %v", onOff))
			return err
		}, nil); err != nil {
			s.Fatal("Failed to get Assistant bluetooth query response: ", err)
		}

		s.Log("Checking Bluetooth status using dbus")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if enabled, err := bluetoothEnabled(ctx); err != nil {
				return testing.PollBreak(err)
			} else if enabled != status {
				return errors.Wrapf(err, "incorrect bluetooth state (expected: %v, actual: %v", status, enabled)
			}
			return nil
		}, nil); err != nil {
			s.Fatal("Failed checking bluetooth status via dbus: ", err)
		}

		// Check if button in the Settings app UI updated to match the actual status.
		// The buttons don't update immediately, so we'll need to poll their statuses.
		// The "aria-pressed" htmlAttribute of the toggle buttons can be used to check the on/off status
		s.Log("Checking bluetooth toggle button status")
		ui := uiauto.New(tconn)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			bluetoothToggle := nodewith.Name("Bluetooth enable").Role(role.ToggleButton)
			if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(bluetoothToggle)(ctx); err != nil {
				testing.PollBreak(err)
			}

			info, err := ui.Info(ctx, bluetoothToggle)
			if err != nil {
				testing.PollBreak(err)
			}
			if info.HTMLAttributes["aria-pressed"] != strconv.FormatBool(status) {
				return errors.Errorf("bluetooth not toggled yet, aria-pressed is %v, expected %v",
					info.HTMLAttributes["aria-pressed"], status)
			}
			return nil
		}, nil); err != nil {
			s.Fatal("Bluetooth button (Settings app) was not toggled by the Assistant: ", err)
		}

		// Check Bluetooth quick setting pod as well.
		s.Log("Checking bluetooth status in Quick Settings")
		if btPodStatus, err := quicksettings.SettingEnabled(ctx, tconn, quicksettings.SettingPodBluetooth); err != nil {
			s.Fatal("Failed to get Bluetooth quick setting status: ", err)
		} else if btPodStatus != status {
			s.Fatal("Bluetooth quick setting pod was not toggled by the Assistant")
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
