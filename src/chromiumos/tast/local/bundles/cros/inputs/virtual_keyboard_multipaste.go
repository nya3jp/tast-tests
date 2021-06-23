// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardMultipaste,
		Desc:         "Test multipaste virtual keyboard functionality",
		Contacts:     []string{"jiwan@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		Pre:          pre.VKEnabledTablet,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
	})
}

func VirtualKeyboardMultipaste(ctx context.Context, s *testing.State) {
	const (
		text1        = "Hello world"
		text2        = "12345"
		expectedText = "Hello world12345"
	)

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Launch inputs test web server.
	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)

	ash.SetClipboard(ctx, tconn, text1)
	ash.SetClipboard(ctx, tconn, text2)

	if err := uiauto.Combine("navigate to multipaste virtual keyboard and paste text",
		its.ClickFieldUntilVKShown(inputField),
		vkbCtx.SwitchToMultipaste(),
		vkbCtx.TapMultipasteItem(text1),
		vkbCtx.TapMultipasteItem(text2),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), expectedText),
	)(ctx); err != nil {
		s.Fatal("Fail to paste text through multipaste virtual keyboard: ", err)
	}
}
