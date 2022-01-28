// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NoSystemUI,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Kiosk configuration starts when set to autologin",
		Contacts: []string{
			"irfedorova@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: fixture.KioskLoggedInAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.KioskLoggedInLacros,
		}},
	})
}

func NoSystemUI(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	testConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "kiosk_NoSystemUI")

	if err := ash.WaitForShelf(ctx, testConn, 30*time.Second); err == nil {
		s.Fatal("Shelf is shown in Kiosk")
	}

	ui := uiauto.New(testConn)

	launcherFinder := nodewith.ClassName("ash/HomeButton").First()
	if err := ui.WaitUntilExists(launcherFinder)(ctx); err == nil {
		s.Fatal("Launcher button is shown in Kiosk")
	} else if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
		s.Fatal("Failed to wait for 'Launcher' button: ", err)
	}

	statusFinder := nodewith.ClassName("UnifiedSystemTrayView").First()
	if err := ui.WaitUntilExists(statusFinder)(ctx); err == nil {
		s.Fatal("Status is shown in Kiosk")
	} else if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
		s.Fatal("Failed to wait for 'Status': ", err)
	}

	toolbarFinder := nodewith.ClassName("ToolbarView").First()
	if err := ui.WaitUntilExists(toolbarFinder)(ctx); err == nil {
		s.Fatal("Toolbar is shown in Kiosk")
	} else if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
		s.Fatal("Failed to wait for 'Toolbar': ", err)
	}

	omniboxFinder := nodewith.ClassName("OmniboxViewViews").Role(role.TextField).First()
	if err := ui.WaitUntilExists(omniboxFinder)(ctx); err == nil {
		s.Fatal("Omnibox is shown in Kiosk")
	} else if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
		s.Fatal("Failed to wait for 'Omnibox': ", err)
	}
}
