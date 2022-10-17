// Package wifi ...
// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Enable and disable WiFi from ChromeOS Settings UI",
		Contacts:     []string{"pathan.jilani@intel.com", "cros-network-health@google.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational", "group:intel-gating"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// UIToggleFromWIFISettings tests enabling/disabling WiFi from the WiFi settings UI in ChromeOS settings.
func UIToggleFromWIFISettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)
	wiFiButton := nodewith.Name("Wi-Fi").Role(role.Button)
	toggleWiFi := nodewith.Name("Wi-Fi enable").Role(role.ToggleButton)

	networkFinder := nodewith.Name("Network").Role(role.Link).Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "Network", ui.Exists(networkFinder)); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	if err := ui.LeftClick(networkFinder)(ctx); err != nil {
		s.Fatal("Failed to left click network tab: ", err)
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill Manager object: ", err)
	}

	wiFiPrevState, err := manager.IsEnabled(ctx, shill.TechnologyWifi)
	if err != nil {
		s.Fatal("Failed to get WiFi state: ", err)
	}

	checkWiFiEnabled := func(ctx context.Context, enable bool) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			wiFiEnabled, err := manager.IsEnabled(ctx, shill.TechnologyWifi)
			if err != nil {
				return errors.Wrap(err, "failed to check if WiFi is enabled")
			}
			if enable && !wiFiEnabled {
				return errors.New("WiFi not available after toggle on")
			}
			if !enable && wiFiEnabled {
				return errors.New("WiFi available after toggle off")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 250 * time.Millisecond})
	}

	defer func(ctx context.Context) {
		s.Log("Cleanup")
		wiFiCurState, err := manager.IsEnabled(ctx, shill.TechnologyWifi)
		if err != nil {
			s.Fatal("Failed to get WiFi state: ", err)
		}
		if wiFiPrevState != wiFiCurState {
			if err := ui.LeftClick(toggleWiFi)(ctx); err != nil {
				s.Fatal("Failed to left click toggleWiFi: ", err)
			}
			if err := checkWiFiEnabled(ctx, wiFiPrevState); err != nil {
				s.Fatal("Failed to check WiFi previous state: ", err)
			}
		}
	}(cleanupCtx)

	const numIterations = 5
	for i := 0; i < numIterations; i++ {
		s.Logf("Iteration %d of %d", i+1, numIterations)
		// Toggle on WiFi button.
		if err := ui.Exists(wiFiButton)(ctx); err != nil {
			if err := ui.LeftClick(toggleWiFi)(ctx); err != nil {
				s.Fatal("Failed to left click toggleWiFi: ", err)
			}
		}
		if err := checkWiFiEnabled(ctx, true); err != nil {
			s.Fatal("Failed to check WiFi enable: ", err)
		}
		// Toggle off WiFi button.
		if err := ui.LeftClick(toggleWiFi)(ctx); err != nil {
			s.Fatal("Failed to left click toggleWiFi: ", err)
		}
		if err := checkWiFiEnabled(ctx, false); err != nil {
			s.Fatal("Failed to check WiFi disable: ", err)
		}
	}
}
