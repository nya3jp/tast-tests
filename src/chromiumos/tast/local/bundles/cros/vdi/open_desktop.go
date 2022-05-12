// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vdi

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/fixtures"
	"chromiumos/tast/testing"
)

type desktopData struct {
	DesktopName   string
	RunDialogKeys string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenDesktop,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test opens Desktop in VDI sessions in user session, Kiosk and MGS",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"cros-engprod-muc@google.com",
		},
		// TODO(b/211600718): Create a separate group not to run tests in parallel.
		// TODO(crbug.com/1293793): Add cleanup for kiosk and add its params.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "citrix",
				Fixture: fixture.CitrixLaunched,
				Val: desktopData{
					DesktopName:   "WindowsServer2019",
					RunDialogKeys: "Search+R",
				},
			},
			{
				Name:    "vmware",
				Fixture: fixture.VmwareLaunched,
				Val: desktopData{
					DesktopName:   "TD-RDS-DESKTOPS",
					RunDialogKeys: "Ctrl+Search+R",
				},
			},
			{
				Name:    "mgs_citrix",
				Fixture: fixture.MgsCitrixLaunched,
				Val: desktopData{
					DesktopName:   "WindowsServer2019",
					RunDialogKeys: "Search+R",
				},
			},
			{
				Name:    "mgs_vmware",
				Fixture: fixture.MgsVmwareLaunched,
				Val: desktopData{
					DesktopName:   "TD-RDS-DESKTOPS",
					RunDialogKeys: "Ctrl+Search+R",
				},
			},
		},
		Data: []string{"toolbar_buttons_icon.png"},
	})
}

func OpenDesktop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	vdi := s.FixtValue().(fixtures.HasVDIConnector).VDIConnector()
	uidetector := s.FixtValue().(fixtures.HasUIDetector).UIDetector()

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer kb.Close()

	isOpened := func(ctx context.Context) error {
		if err := uidetector.WithTimeout(60 * time.Second).WaitUntilExists(uidetection.CustomIcon(s.DataPath("toolbar_buttons_icon.png")))(ctx); err != nil {
			return errors.Wrap(err, "failed waiting for the toolbar buttons icon to appear")
		}

		return nil
	}

	desktopToOpen := s.Param().(desktopData).DesktopName
	keysToOpenRunDialog := s.Param().(desktopData).RunDialogKeys

	if err := vdi.SearchAndOpenApplication(ctx, kb, desktopToOpen, isOpened)(ctx); err != nil {
		s.Fatalf("Failed to open %v app: %v", desktopToOpen, err)
	}

	// Move focus on Windows desktop.
	if err := kb.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Failed to execute Tab command: ", err)
	}

	// Invoke opening the run dialog box.
	if err := ui.RetryUntil(
		kb.AccelAction(keysToOpenRunDialog),
		uidetector.WithTimeout(10*time.Second).WaitUntilExists(uidetection.Word("Run").First()),
	)(ctx); err != nil {
		s.Error("Failed to invoke opening the run dialog box: ", err)
	}

	// Cleanup is to close the desktop window.

	w, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the active window: ", err)
	}
	if err := w.CloseWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to close the active window: ", err)
	}

	if err := vdi.ResetSearch(ctx, kb); err != nil {
		s.Fatal("Was not able to reset search results: ", err)
	}
}
