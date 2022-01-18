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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/vdi/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenDesktop,
		Desc: "Test opens Desktop in VDI sessions in user session, Kiosk and MGS",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"cros-engprod-muc@google.com",
		},
		// TODO(b/211600718): Create a separate group not to run tests in parallel.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "citrix",
				Fixture: fixture.CitrixLaunched,
			},
			{
				Name:    "vmware",
				Fixture: fixture.VmwareLaunched,
			},
			{
				Name:    "kiosk_citrix",
				Fixture: fixture.KioskCitrixLaunched,
			},
			// b/207122370
			// Vmware in Kiosk mode does not receive Ctrl+w to close tab.
			{
				Name:    "kiosk_vmware",
				Fixture: fixture.KioskVmwareLaunched,
			},
			{
				Name:    "mgs_citrix",
				Fixture: fixture.MgsCitrixLaunched,
			},
			{
				Name:    "mgs_vmware",
				Fixture: fixture.MgsVmwareLaunched,
			},
		},
		Data: []string{"start_button_icon.png"},
	})
}

func OpenDesktop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	vdi := s.FixtValue().(fixtures.HasVDIConnector).VDIConnector()
	uidetector := s.FixtValue().(fixtures.HasUIDetector).UIDetector()

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	const desktopToOpen = "On-Prem Desktop EMEA"

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get a keyboard")
	}
	defer kb.Close()

	isOpened := func(ctx context.Context) error {
		if err := uidetector.WithTimeout(60 * time.Second).WaitUntilExists(uidetection.CustomIcon(s.DataPath("start_button_icon.png")))(ctx); err != nil {
			return errors.Wrap(err, "failed waiting for start button icon to appear")
		}

		return nil
	}

	if err := vdi.SearchAndOpenApplication(ctx, kb, desktopToOpen, isOpened)(ctx); err != nil {
		s.Fatalf("Failed to open %v app: %v", desktopToOpen, err)
	}

	// Move focus on Windows desktop.
	if err := kb.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Failed to execute Tab command: ", err)
	}
	// Press start button to open the start Menu.
	if err := kb.Accel(ctx, "Ctrl+Esc"); err != nil {
		s.Fatal("Failed to open the start Menu with Ctrl+Esc: ", err)
	}
	// Type some words to open the search menu.
	if err := kb.Type(ctx, "test searching on windows"); err != nil {
		s.Fatal("Failed to type words: ", err)
	}
	// Wait till we find "Best match" block to indicate that the search menu is open.
	if err := uidetector.WaitUntilExists(uidetection.TextBlock([]string{"Best", "match"}).First())(ctx); err != nil {
		s.Fatal("Did not find text block confirming windows search menu is open: ", err)
	}
	// Cleanup the open menu.
	if err := kb.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Failed to open the start Menu with Esc: ", err)
	}
}
