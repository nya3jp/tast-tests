// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WifiQueries,
		Desc:         "Tests toggling WiFi using Assistant queries",
		Contacts:     []string{"kyleshima@chromium.org", "bhansknecht@chromium.org", "meilinw@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "shill-wifi"},
		Pre:          assistant.VerboseLoggingEnabled(),
	})
}

// WifiQueries tests that Assistant queries can be used to toggle WiFi on/off
func WifiQueries(ctx context.Context, s *testing.State) {
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

	// Open the Settings window, where we can verify Wifi button status
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

		s.Log("Turning wifi ", onOff)
		// assistant.SendTextQuery sometimes times out after the assistant UI is closed,
		// so poll to ensure the queries go through.
		// todo: remove polling when crbug/1080363 is fixed
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			_, err := assistant.SendTextQuery(ctx, tconn, fmt.Sprintf("turn wifi %v", onOff))
			return err
		}, nil); err != nil {
			s.Fatal("Failed to get Assistant wifi query response: ", err)
		}

		s.Log("Checking wifi status")
		if err := expectWifiEnabled(ctx, status); err != nil {
			s.Fatal("Wifi status was not changed by the assistant query: ", err)
		}

		// Check if button in the Settings app UI updated to match the actual status.
		// The buttons don't update immediately, so we'll need to poll their statuses.
		// The "aria-pressed" htmlAttribute of the toggle buttons can be used to check the on/off status
		s.Log("Checking wifi toggle button status")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			params := ui.FindParams{Name: "Wi-Fi enable", Role: ui.RoleTypeToggleButton}
			wifiToggle, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer wifiToggle.Release(ctx)

			if wifiToggle.HTMLAttributes["aria-pressed"] != strconv.FormatBool(status) {
				return errors.Errorf("wifi not toggled yet, aria-pressed is %v, expected %v",
					wifiToggle.HTMLAttributes["aria-pressed"], status)
			}
			return nil
		}, nil); err != nil {
			s.Fatal("WiFi toggle button in the Settings app was not toggled by the Assistant query: ", err)
		}
	}
}

// expectWifiEnabled uses shill to check if Wifi is in the expected state
func expectWifiEnabled(ctx context.Context, expectEnabled bool) error {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get shill manager")
	}
	watcher, err := m.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer watcher.Close(ctx)

	for {
		prop, err := m.GetProperties(ctx)
		technologies, err := prop.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
		if err != nil {
			return errors.Wrap(err, "failed to get enabled technologies")
		}

		found := false
		for _, t := range technologies {
			if t == string(shill.TechnologyWifi) {
				found = true
				break
			}
		}
		if expectEnabled == found {
			return nil
		}
		// If the Wifi status is not what we expect, check again when the enabled technologies property changes.
		if _, err := watcher.WaitAll(ctx, shillconst.ManagerPropertyEnabledTechnologies); err != nil {
			return errors.Wrap(err, "failed waiting for enabled technologies to change")
		}
	}
}
