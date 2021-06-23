// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingApps,
		Desc:         "Checks that the virtual keyboard works in apps",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}}})
}

func VirtualKeyboardTypingApps(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	// Input string should start with lower case letter because VK layout is not auto-capitalized in the settings search bar.
	const typingKeys = "language"

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "VirtualKeyboardTypingApps.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}

	app := apps.Settings
	s.Logf("Launching %s", app.Name)
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		s.Fatalf("Failed to launch %s: %c", app.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
	}

	vkbCtx := vkb.NewContext(cr, tconn)
	searchFieldFinder := nodewith.Role(role.SearchBox).Name("Search settings")
	if err := uiauto.Combine("test virtual keyboard input in settings app",
		vkbCtx.ClickUntilVKShown(searchFieldFinder),
		vkbCtx.WaitForDecoderEnabled(true),
		vkbCtx.TapKeys(strings.Split(typingKeys, "")),
		// Hide virtual keyboard to submit candidate.
		vkbCtx.HideVirtualKeyboard(),
		// Validate text.
		util.WaitForFieldTextToBe(tconn, searchFieldFinder, typingKeys),
	)(ctx); err != nil {
		s.Fatal("Failed to verify virtual keyboard input in settings: ", err)
	}
}
