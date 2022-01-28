// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/ctxutil"
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
			Timeout: 1 * time.Minute,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.KioskLoggedInLacros,
			Timeout:           1 * time.Minute,
		}},
	})
}

func NoSystemUI(ctx context.Context, s *testing.State) {
	// Default timeout for UI interactions.
	const defaultUITimeout = 5 * time.Second

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	testConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "kiosk_NoSystemUI")

	if err := ash.WaitForShelf(ctx, testConn, defaultUITimeout); err == nil {
		s.Fatal("Shelf is shown in Kiosk")
	}

	ui := uiauto.New(testConn).WithTimeout(defaultUITimeout)

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
			finder := nodewith.HasClass(param.className).First()
			if err := ui.WaitUntilExists(finder)(ctx); err == nil {
				s.Fatal(param.errorElementName, " is shown in Kiosk")
			} else if !strings.Contains(err.Error(), nodewith.ErrNotFound) {
				s.Fatal("Failed to wait for ", param.errorElementName, ": ", err)
			}
		})
	}
}
