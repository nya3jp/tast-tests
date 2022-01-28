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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NoSystemUI,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that no system UI is shown in PWA Kiosk",
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
	// Default timeout for UI interactions.
	const DefaultUITimeout = 5 * time.Second

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	testConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "kiosk_NoSystemUI")

	if err := ash.WaitForShelf(ctx, testConn, DefaultUITimeout); err == nil {
		s.Fatal("Shelf is shown in Kiosk")
	}

	ui := uiauto.New(testConn).WithTimeout(DefaultUITimeout)

	for _, param := range []struct {
		// class name for nodewith.finder
		className string
		// element name that would be included to error message
		errorElementName string
	}{
		{
			className:        "ash/HomeButton",
			errorElementName: "'Launcher' button",
		},
		{
			className:        "UnifiedSystemTray",
			errorElementName: "'Status'",
		},
		{
			className:        "ToolbarView",
			errorElementName: "'Toolbar'",
		},
		{
			className:        "OmniboxViewViews",
			errorElementName: "'Omnibox'",
		},
	} {
		s.Run(ctx, param.className, func(ctx context.Context, s *testing.State) {
			finder := nodewith.ClassName(param.className).First()
			if err := ui.WaitUntilExists(finder)(ctx); err == nil {
				s.Fatal(param.errorElementName, " is shown in Kiosk")
			} else if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
				s.Fatal("Failed to wait for ", param.errorElementName, ": ", err)
			}
		})
	}
}
