// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vdi

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenChromeApp,
		Desc: "Test opens Google Chrome application in VDI sessions in user session, Kiosk and MGS",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		// TODO: Create separate group not to run tests in parallel. Reason
		// being - when VDI is accessed and the same user logs in from
		// elsewhere then the previous session behaves weirdly e.g. cannot
		// open applications (VMware).
		// Another reason is not to use the CaPSE infrastructure with accounts
		// used by them.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "citrix",
				Fixture: fixture.CitrixLaunched,
				Val:     false,
			},
			{
				Name:    "vmware",
				Fixture: fixture.VmwareLaunched,
				Val:     false,
			},
			{
				Name:    "kiosk_citrix",
				Fixture: fixture.KioskCitrixLaunched,
				Val:     true,
			},
			// Vmware does not receive Ctrl+w to close tab.
			// {
			// 	Name:    "kiosk_vmware",
			// 	Fixture: fixture.KioskVmwareLaunched,
			// 	Val:     true,
			// },
			{
				Name:    "mgs_citrix",
				Fixture: fixture.MgsCitrixLaunched,
				Val:     false,
			},
			{
				Name:    "mgs_vmware",
				Fixture: fixture.MgsVmwareLaunched,
				Val:     false,
			},
		},
	})
}

func OpenChromeApp(ctx context.Context, s *testing.State) {
	vdi := s.FixtValue().(fixtures.HasVdiConnector).VdiConnector()
	cr := s.FixtValue().(fixtures.HasChrome).Chrome()
	uidetector := s.FixtValue().(fixtures.HasUIDetector).UIDetector()
	kioskMode := s.Param().(bool)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	k, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer k.Close()

	const appToOpen = "Chrome"
	if err := vdi.SearchAndOpenApplication(ctx, k, appToOpen)(ctx); err != nil {
		s.Fatalf("Failed to open %v app: %v", appToOpen, err)
	}
	defer vdi.ResetSearch(ctx, k)
	if kioskMode {
		defer k.Accel(ctx, "Ctrl+w") // Close the tab. This is not passed to Vmware Horizon
		defer k.Accel(ctx, "Tab")    // Move focus on the Chrome.
		defer k.Accel(ctx, "Tab")    // Move focus on the Chrome.
	} else {
		defer k.Accel(ctx, "Ctrl+w") // Close the tab.
		defer k.Accel(ctx, "Tab")    // Move focus on the Chrome.
	}

	if !kioskMode {
		// Wait for actual application window to open.
		if err := ash.WaitForCondition(ctx, tconn,
			func(w *ash.Window) bool {
				return strings.Contains(w.Title, appToOpen)
			},
			&testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			s.Fatal("Failed to find window for: ", err)
		}
	}

	// Use First() as in VMWare mouse hovers over the tab showing its ballon
	// tip containing "New tab".
	if err := uidetector.WaitUntilExists(uidetection.TextBlock([]string{"New", "tab"}).First())(ctx); err != nil {
		s.Fatal("Did not find text block confirming Chrome has started: ", err)
	}
}
