// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/crossdevice/phonehub"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RecentTabs,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that recently opened Chrome tabs on Android appear in Phone Hub",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboardedAllFeatures",
	})
}

// RecentTabs tests that recently opened Chrome tabs on Android appear in Phone Hub and can be opened in Chrome on CrOS.
func RecentTabs(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*crossdevice.FixtData).Chrome
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice

	// Opt in to Chrome Sync. This is required for recent tabs to appear in Phone Hub.
	if err := androidDevice.EnableChromeSync(ctx); err != nil {
		s.Fatal("Failed to enable Chrome Sync on Android: ", err)
	}

	// Open some tabs on Android.
	urls := []string{"chromium.org", "google.com"}
	for _, url := range urls {
		if err := androidDevice.LaunchChromeAtURL(ctx, url); err != nil {
			s.Fatalf("Failed open Chrome tab for URL %v: %v", url, err)
		}
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "recent_tabs_failure")

	// Wait for the chips in Phone Hub to update with the recently opened tabs.
	if err := phonehub.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Phone Hub: ", err)
	}
	if err := testing.Poll(ctx, func(context.Context) error {
		chips, err := phonehub.RecentTabChips(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get recent tab chips")
		}
		for _, url := range urls {
			found := false
			for _, chip := range chips {
				if strings.Contains(chip.URL, url) {
					found = true
				}
			}
			if !found {
				return errors.Errorf("chip for URL %v not found", url)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("Timed out waiting for Phone Hub recent tab chips to update: ", err)
	}

	// Click each of the chips and make sure they open Chrome to the correct URL.
	chips, err := phonehub.RecentTabChips(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get recent tab chips: ", err)
	}
	ui := uiauto.New(tconn)
	for _, chip := range chips {
		if err := phonehub.Show(ctx, tconn); err != nil {
			s.Fatal("Failed to open Phone Hub: ", err)
		}
		if err := ui.LeftClick(chip.Finder)(ctx); err != nil {
			s.Fatalf("Failed to click chip for %v: %v", chip.URL, err)
		}
		c, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(chip.URL))
		if err != nil {
			s.Fatalf("Failed to find Chrome window for %v: %v", chip.URL, err)
		}
		defer c.Close()
		defer c.CloseTarget(ctx)
	}
}
