// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIToggleFromWIFISettings,
		Desc:         "Enable and disable WIFI from ChromeOS Settings UI",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// UIToggleFromWIFISettings tests enabling/disabling Wifi from the Wifi settings UI in ChromeOS settings.
func UIToggleFromWIFISettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)
	networkURL := nodewith.Name("Network").Role(role.Link)
	wifiButton := nodewith.Name("Wi-Fi").Role(role.Button)
	toggleWifi := nodewith.Name("Wi-Fi enable").Role(role.ToggleButton)

	// launch settings page
	if _, err = ossettings.LaunchAtPageURL(ctx, tconn, cr, "Network", ui.Exists(toggleWifi)); err != nil {
		s.Fatal("Failed to bring up Wifi os settings page: ", err)
	}
	if err := ui.LeftClick(networkURL)(ctx); err != nil {
		s.Fatal("Failed to left click networkURL: ", err)
	}
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create Manager object: ", err)
	}

	wifiToggle := func(ctx context.Context, toggle string) error {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if wifiEnabled, err := manager.IsEnabled(ctx, shill.TechnologyWifi); err != nil {
				return errors.Wrap(err, "error calling IsEnabled")
			} else if toggle == "ON" {
				if !wifiEnabled {
					return errors.New("wifi not available after toggle on")
				}

			} else if toggle == "OFF" {
				if wifiEnabled {
					return errors.New("wifi available after toggle off")
				}
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
			return err
		}
		return err
	}
	const numIterations = 5
	for i := 0; i < numIterations; i++ {
		s.Logf("Iteration %d of %d", i+1, numIterations)
		// toggle on wifi button
		if err := ui.Exists(wifiButton)(ctx); err != nil {
			if err := ui.LeftClick(toggleWifi)(ctx); err != nil {
				s.Fatal("Failed to left click toggleWifi: ", err)
			}
		}
		// validating wifi ON
		if err := wifiToggle(ctx, "ON"); err != nil {
			s.Fatal(err)
		}
		// toggle off wifi button
		if err := ui.LeftClick(toggleWifi)(ctx); err != nil {
			s.Fatal("Failed to left click toggleWifi: ", err)
		}
		// validating wifi ON
		if err := wifiToggle(ctx, "OFF"); err != nil {
			s.Fatal(err)
		}
	}
}
