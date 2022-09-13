// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OnscreenKeyboard,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Enable On-screen keyboard",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func OnscreenKeyboard(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := quicksettings.ShowWithRetry(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Failed to open Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	if err := quicksettings.OpenSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to open the Settings App from Quick Settings: ", err)
	}

	// Confirm that the Settings app is open by checking for the search box.
	if err := uiauto.New(tconn).WaitUntilExists(ossettings.SearchBoxFinder)(ctx); err != nil {
		s.Fatal("Waiting for Settings app search box failed: ", err)
	}

	settings := ossettings.New(tconn)
	defer settings.Close(ctx)

	manageAccessibility := nodewith.Name("Manage accessibility features Enable accessibility features").Role(role.Link)
	onscreenButton := nodewith.Name("Enable on-screen keyboard").Role(role.ToggleButton).Focusable()

	if err := uiauto.Combine("Enable on-screen keyboard, using Accessibility",
		settings.FocusAndWait(ossettings.Advanced),
		settings.LeftClick(ossettings.Advanced),
		settings.WaitUntilExists(ossettings.Advanced.State(state.Expanded, true)),
		settings.FocusAndWait(ossettings.Accessibility),
		settings.LeftClick(ossettings.Accessibility),
		settings.LeftClick(manageAccessibility),
		settings.FocusAndWait(onscreenButton),
		settings.LeftClick(onscreenButton),
	)(ctx); err != nil {
		s.Log("Failed to enable on-screen keyboard: ", err)
	}

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	ui := uiauto.New(tconn)
	vkbCtx := vkb.NewContext(cr, tconn)

	vkNode := nodewith.Name("Chrome OS Virtual Keyboard").Role(role.Keyboard)

	if err := uiauto.Combine("Verify on-screen keyboard voice input and keys",
		ui.LeftClick(nodewith.ClassName("OmniboxViewViews").Role(role.TextField)),
		ui.WaitUntilExists(vkNode),
		vkbCtx.SwitchToVoiceInput(),
		vkbCtx.HideVirtualKeyboard(),
		ui.LeftClick(nodewith.ClassName("OmniboxViewViews").Role(role.TextField)),
		ui.WaitUntilExists(vkNode),
		vkbCtx.TapKeys([]string{"t", "a", "s", "t"}),
	)(ctx); err != nil {
		s.Log("Failed to verify on-screen keyboard voice input and keys: ", err)
	}
	if err := vkbCtx.HideVirtualKeyboard()(cleanupCtx); err != nil {
		s.Error("Failed to Hide Virtual Keyboard: ", err)
	}
}
