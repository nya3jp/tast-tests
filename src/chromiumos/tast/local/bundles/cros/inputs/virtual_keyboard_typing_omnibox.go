// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingOmnibox,
		Desc:         "Checks that the virtual keyboard works in Chrome browser omnibox",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: pre.InputsStableModels,
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "exp",
			Pre:               pre.VKEnabledTabletExp,
			ExtraSoftwareDeps: []string{"gboard_decoder"},
			ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
		}}})
}

func VirtualKeyboardTypingOmnibox(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	const typingKeys = "go"
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to connect to test page: ", err)
	}
	defer conn.Close()

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
