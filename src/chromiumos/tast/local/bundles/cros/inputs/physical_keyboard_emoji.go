// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEmoji,
		Desc:         "Checks that right click input field and select emoji with physical keyboard",
		Contacts:     []string{"jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraAttr:         []string{"group:input-tools-upstream"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kodama", "kefka")),
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}}})
}

func PhysicalKeyboardEmoji(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	const (
		inputField = testserver.TextInputField
		emojiChar  = "😂"
	)

	emojiMenuFinder := nodewith.NameStartingWith("Emoji")
	emojiPickerFinder := nodewith.Name("Emoji Picker").Role(role.RootWebArea)
	emojiCharFinder := nodewith.Name(emojiChar).First().Ancestor(emojiPickerFinder)

	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := uiauto.Combine("verify quick emoji input",
		// Right click input to trigger context menu and select Emoji.
		ui.RightClick(inputField.Finder()),
		ui.LeftClick(emojiMenuFinder),
		// Select item from emoji picker.
		ui.LeftClick(emojiCharFinder),
		// Wait for input value to test emoji.
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), emojiChar),
	)(ctx); err != nil {
		s.Fatal("Failed to verify emoji picker: ", err)
	}
}
