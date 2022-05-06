// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardLockScreen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the virtual keyboard works on lock screen",
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		Contacts:     []string{"essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Params: []testing.Param{
			{
				Name:    "tablet",
				Fixture: fixture.TabletVK,
			},
			{
				Name:    "clamshell",
				Fixture: fixture.ClamshellVK,
			},
		},
	})
}

func VirtualKeyboardLockScreen(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext
	uc.SetTestName(s.TestName())

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, s.TestName())

	// Add text to clipboard.
	if err := ash.SetClipboard(ctx, tconn, "text in clipboard"); err != nil {
		s.Fatal("Failed to set clipboard: ", err)
	}

	osSetting, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch OS settings: ", err)
	}
	defer osSetting.Close(ctx)

	if err := osSetting.AddFakeVPNSetting()(ctx); err != nil {
		s.Fatal("Failed to add fake vpn: ", err)
	}

	s.Log("Locking the ChromeOS screen")
	if err := quicksettings.LockScreen(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen on ChromeOS: ", err)
	}

	vkbCtx := vkb.NewContext(cr, tconn)
	// VK is automatically shown due to password field get focused.
	// It does nothing is VK is not shown.
	if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
		s.Fatal("Failed to hide virtual keyboard: ", err)
	}

	if err := quicksettings.TriggerAddingVPNDialog(tconn)(ctx); err != nil {
		s.Fatal("Failed to trigger adding VPN dialog: ", err)
	}

	serviceNameInputFieldFinder := nodewith.Name("Service name").Role(role.TextField)

	// Using a misspell word to validate that auto-correction does not engage.
	tapKeys := "helol"
	if err := uiauto.Combine("Verify VK input",
		vkbCtx.ClickUntilVKShown(serviceNameInputFieldFinder),
		vkbCtx.TapKeysIgnoringCase(strings.Split(tapKeys, "")),
		vkbCtx.TapKeyIgnoringCase("Space"),
		util.WaitForFieldTextToBeIgnoringCase(tconn, serviceNameInputFieldFinder, tapKeys+" "),
	)(ctx); err != nil {
		s.Fatalf("Failed to verify VK input in %v field: %v", serviceNameInputFieldFinder, err)
	}
}
