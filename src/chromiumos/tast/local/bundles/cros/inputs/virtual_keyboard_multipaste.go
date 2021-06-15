// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardMultipaste,
		Desc:         "Test multipaste virtual keyboard functionality",
		Contacts:     []string{"jiwan@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		Pre:          pre.VKEnabledTablet,
	})
}

func VirtualKeyboardMultipaste(ctx context.Context, s *testing.State) {
	const (
		initialText  = "Hello world"
		expectedText = "Hello worldHello world"
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

	if err := uiauto.Combine("input intial text and copy it",
		// Click left top to focus on the input area.
		its.ClickFieldAndWaitForActive(inputField),
		// Type string.
		keyboard.TypeAction(initialText),
		// Press ctrl+x and ctrl+s to save.
		keyboard.AccelAction("ctrl+A"),
		keyboard.AccelAction("ctrl+C"),
		keyboard.AccelAction("end"))(ctx); err != nil {
		s.Fatal("Fail to input intial text and copy it: ", err)
	}

	// Show VK.
	if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
		s.Fatal("Failed to show VK: ", err)
	}

	// Switch to handwriting layout.
	if err := vkbCtx.SwitchToMultipaste()(ctx); err != nil {
		s.Fatal("Failed to switch to multipaste: ", err)
	}

	if err := vkbCtx.TapMultipasteItem(initialText)(ctx); err != nil {
		s.Fatal("Failed to tap multipaste item: ", err)
	}

	if err := its.WaitForFieldValueToBe(inputField, expectedText)(ctx); err != nil {
		s.Fatal("Fail to validate input field content: ", err)
	}
}
