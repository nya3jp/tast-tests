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
		Func:         VirtualKeyboardTypingOmnibox,
		Desc:         "Checks that the virtual keyboard works in Chrome browser omnibox",
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

func VirtualKeyboardTypingOmnibox(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	const typingKeys = "go"
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "VirtualKeyboardTypingOmnibox.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}

	// Warning: Please do not launch Browser via cr.NewConn(ctx, "")
	// to test omnibox typing. It might be indeterminate whether default url string
	// "about:blank" is highlighted or not.
	// In that case, typing test can either replace existing url or insert into it.
	// A better way to do it is launching Browser from launcher, url is empty by default.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatalf("Failed to launch %s: %s", apps.Chrome.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %s", apps.Chrome.Name, err)
	}

	omniboxFinder := nodewith.Role(role.TextField).Attribute("inputType", "url")
	vkbCtx := vkb.NewContext(cr, tconn)

	if err := uiauto.Combine("verify virtual keyboard input on omnibox",
		vkbCtx.ClickUntilVKShown(omniboxFinder),
		vkbCtx.TapKeys(strings.Split(typingKeys, "")),
		// Hide virtual keyboard to submit candidate.
		vkbCtx.HideVirtualKeyboard(),
		// Validate text.
		util.WaitForFieldTextToBe(tconn, omniboxFinder, typingKeys),
	)(ctx); err != nil {
		s.Fatal("Failed to verify virtual keyboard input on omnibox: ", err)
	}
}
