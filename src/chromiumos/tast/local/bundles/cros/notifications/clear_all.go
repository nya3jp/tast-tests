// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ClearAll,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that the 'Clear all' button dismisses all notifications",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"amehfooz@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

// ClearAll tests that several notifications can be dismissed with the 'Clear all' button.
func ClearAll(ctx context.Context, s *testing.State) {
	const uiTimeout = 30 * time.Second

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Setup a browser.
	bt := s.Param().(browser.Type)
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)
	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatalf("Failed to create Test API connection for %v browser: %v", bt, err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(ctx)

	const baseTitle = "TestNotification"
	const n = 10
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("%s%d", baseTitle, i)
		if _, err := browser.CreateTestNotification(ctx, bTconn, browser.NotificationTypeBasic, title, "blahhh"); err != nil {
			s.Fatalf("Failed to create test notification %v: %v", i, err)
		}
		if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(title)); err != nil {
			s.Fatalf("Failed waiting for %v: %v", title, err)
		}
	}

	// Open Quick Settings to ensure the 'Clear all' button is available.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	ui := uiauto.New(tconn)
	clearAll := nodewith.Name("Clear all").Role(role.StaticText)
	if err := ui.WithTimeout(uiTimeout).LeftClick(clearAll)(ctx); err != nil {
		s.Fatal("Failed to click 'Clear all' button: ", err)
	}

	// Wait until all notifications and the 'Clear all' button are gone.
	// The notification names change based on their title and content, so partially match the name attribute.
	r, err := regexp.Compile(baseTitle)
	if err != nil {
		s.Fatal("Failed to compile notification regex: ", err)
	}
	notification := nodewith.ClassName("MessageView").Attribute("name", r)
	if err := ui.WithTimeout(uiTimeout).WaitUntilGone(notification)(ctx); err != nil {
		s.Fatal("Failed waiting for notifications to be dismissed: ", err)
	}
	if err := ui.WithTimeout(uiTimeout).WaitUntilGone(clearAll)(ctx); err != nil {
		s.Fatal("Failed waiting for 'Clear all' button to be gone: ", err)
	}
}
