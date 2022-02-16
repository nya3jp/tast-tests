// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ManagedDeviceInfo,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that the Quick Settings managed device info is displayed correctly",
		Contacts: []string{
			"leandre@chromium.org",
			"amehfooz@chromium.org",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromePolicyLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosPolicyLoggedIn",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

// ManagedDeviceInfo tests that the Quick Settings managed device info is displayed correctly.
func ManagedDeviceInfo(ctx context.Context, s *testing.State) {
	const uiTimeout = 10 * time.Second

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	// Setup a browser.
	bt := s.Param().(browser.Type)
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Check if management information is shown.
	ui := uiauto.New(tconn)
	managedBtn := quicksettings.ManagedInfoView
	if err := ui.WithTimeout(uiTimeout).WaitUntilExists(managedBtn)(ctx); err != nil {
		s.Fatal("Failed to find managed info button: ", err)
	}

	// Check if the information contains the managed domain name or indication that the device is "enterprise managed" (depending on test account configuration).
	info, err := ui.Info(ctx, managedBtn)
	if err != nil {
		s.Fatal("Failed to get management information button info: ", err)
	}
	if !strings.Contains(info.Name, "managedchrome.com") && !strings.Contains(info.Name, "enterprise managed") {
		s.Fatalf("Managed info string: %q, expected containing management domain name or enterprise managed indication", info.Name)
	}

	if err := ui.LeftClick(managedBtn)(ctx); err != nil {
		s.Fatal("Failed to click management information button: ", err)
	}

	// Check if management page is open after clicking the button.
	if _, err := br.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://management/")); err != nil {
		s.Fatal("Management page did not open: ", err)
	}
}
