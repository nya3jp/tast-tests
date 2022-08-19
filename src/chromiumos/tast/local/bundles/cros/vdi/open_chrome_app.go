// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vdi

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/fixtures"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenChromeApp,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test opens Google Chrome application in VDI sessions in user session, Kiosk and MGS",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"cros-engprod-muc@google.com",
		},
		// TODO(b/211600718): Create a separate group not to run tests in parallel.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		// For now limit execution to few models.
		// TODO (b/238736164): Crete dedicated suite.
		HardwareDeps: hwdep.D(hwdep.Model("foob360", "helios", "eve", "kohaku", "morphius")),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:      "citrix",
				Fixture:   fixture.CitrixLaunched,
				ExtraAttr: []string{"group:mainline", "informational"},
			},
			{
				Name:    "vmware",
				Fixture: fixture.VmwareLaunched,
			},
			{
				Name:      "kiosk_citrix",
				Fixture:   fixture.KioskCitrixLaunched,
				ExtraAttr: []string{"group:mainline", "informational"},
			},
			// b/207122370
			// Vmware in Kiosk mode does not receive Ctrl+w to close tab.
			{
				Name:    "kiosk_vmware",
				Fixture: fixture.KioskVmwareLaunched,
			},
			{
				Name:      "mgs_citrix",
				Fixture:   fixture.MgsCitrixLaunched,
				ExtraAttr: []string{"group:mainline", "informational"},
			},
			{
				Name:    "mgs_vmware",
				Fixture: fixture.MgsVmwareLaunched,
			},
		},
	})
}

func OpenChromeApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	vdi := s.FixtValue().(fixtures.HasVDIConnector).VDIConnector()
	kioskMode := s.FixtValue().(fixtures.IsInKioskMode).InKioskMode()
	uidetector := s.FixtValue().(fixtures.HasUIDetector).UIDetector()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	const appToOpen = "Chrome"

	isOpened := func(ctx context.Context) error {
		if !kioskMode {
			// Wait for actual application window to open.
			if err := ash.WaitForCondition(ctx, tconn,
				func(w *ash.Window) bool {
					return strings.Contains(w.Title, appToOpen)
				},
				&testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatalf("Failed to find %s window: %v", appToOpen, err)
			}
		}

		// Use First() as in VMWare mouse hovers over the tab showing its ballon
		// tip containing "New tab".
		textBlock := []string{"New", "tab"}
		if err := uidetector.WaitUntilExists(uidetection.TextBlock(textBlock).First())(ctx); err != nil {
			s.Fatalf("Did not find text block %v confirming %s has started: %v", textBlock, appToOpen, err)
		}
		return nil
	}

	if err := vdi.SearchAndOpenApplication(ctx, appToOpen, isOpened)(ctx); err != nil {
		s.Fatalf("Failed to open %v app: %v", appToOpen, err)
	}
}
