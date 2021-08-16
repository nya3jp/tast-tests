// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardChangeInput,
		Desc:         "Checks that changing input method in different ways on physical keyboard",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Pre:               pre.NonVKClamshell,
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "informational",
			Pre:               pre.NonVKClamshell,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}},
	})
}

func PhysicalKeyboardChangeInput(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	defaultInputMethod := ime.EnglishUS
	inputMethod1 := ime.Japanese
	inputMethod2 := ime.SpanishSpain

	if err := uiauto.Combine("add new input methods",
		inputMethod1.Install(tconn),
		inputMethod2.Install(tconn),
	)(ctx); err != nil {
		s.Fatal("Failed to add new input method: ", err)
	}

	s.Log("Switch input method with keybaord shortcut Ctrl+Shift+Space")
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	if err := uiauto.Combine("switch IME with shortcut",
		// Ctrl + Shift + Space changes IME in rotation.
		keyboard.AccelAction("Ctrl+Shift+Space"),
		inputMethod1.WaitUntilActivated(tconn),
		keyboard.AccelAction("Ctrl+Shift+Space"),
		inputMethod2.WaitUntilActivated(tconn),
		keyboard.AccelAction("Ctrl+Shift+Space"),
		defaultInputMethod.WaitUntilActivated(tconn),
		// Ctrl + Space changes to the most recent IME.
		keyboard.AccelAction("Ctrl+Space"),
		inputMethod2.WaitUntilActivated(tconn),
		keyboard.AccelAction("Ctrl+Space"),
		defaultInputMethod.WaitUntilActivated(tconn),
	)(ctx); err != nil {
		s.Fatal("Failed to switch input method: ", err)
	}

	ui := uiauto.New(tconn)
	capsOnImageFinder := nodewith.Name("CAPS LOCK is on").Role(role.Image)

	if err := uiauto.Combine("caps lock with shortcut",
		// Alt + Search locks caps.
		keyboard.AccelAction("Alt+Search"),
		ui.WaitUntilExists(capsOnImageFinder),

		// Shift to unlock.
		keyboard.AccelAction("Shift"),
		ui.WaitUntilGone(capsOnImageFinder),
	)(ctx); err != nil {
		s.Fatal("Failed to validate caps lock: ", err)
	}
}
