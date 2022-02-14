// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardMultipaste,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test multipaste virtual keyboard functionality",
		Contacts:     []string{"jiwan@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		Pre:          pre.VKEnabledTablet,
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "informational",
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels, hwdep.SkipOnPlatform("puff", "fizz")),
			ExtraAttr:         []string{"informational"},
		}},
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
	uc := s.PreValue().(pre.PreData).UserContext

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
	touchCtx, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Fail to get touch screen: ", err)
	}
	defer touchCtx.Close()

	ash.SetClipboard(ctx, tconn, text1)
	ash.SetClipboard(ctx, tconn, text2)

	if err := uc.RunAction(ctx, "Input from VK multipaste clipboard",
		uiauto.Combine("navigate to multipaste virtual keyboard and paste text",
			its.ClickFieldUntilVKShown(inputField),
			vkbCtx.SwitchToMultipaste(),
			vkbCtx.TapMultipasteItem(text1),
			vkbCtx.TapMultipasteItem(text2),
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), expectedText),
		),
		&useractions.UserActionCfg{
			Tags:       []useractions.ActionTag{useractions.ActionTagMultiPaste},
			Attributes: map[string]string{useractions.AttributeInputField: string(inputField)},
		},
	); err != nil {
		s.Fatal("Fail to paste text through multipaste virtual keyboard: ", err)
	}

	ui := uiauto.New(tconn)

	if err := uc.RunAction(ctx, "select then de-select item in VK multipaste clipboard",
		uiauto.Combine("Select then de-select item in multipaste virtual keyboard",
			touchCtx.LongPress(vkb.MultipasteItemFinder.Name(text1)),
			ui.WithTimeout(3*time.Second).WaitUntilExists(vkb.KeyFinder.ClassName("trash-button")),
			vkbCtx.TapMultipasteItem(text1),
			ui.WithTimeout(3*time.Second).WaitUntilGone(vkb.KeyFinder.ClassName("trash-button"))),
		&useractions.UserActionCfg{
			Tags:       []useractions.ActionTag{useractions.ActionTagMultiPaste},
			Attributes: map[string]string{useractions.AttributeInputField: string(inputField)},
		},
	); err != nil {
		s.Fatal("Fail to long press to select and delete item: ", err)
	}

	if err := uc.RunAction(ctx, "remove item in VK multipaste clipboard",
		vkbCtx.DeleteMultipasteItem(touchCtx, text1),
		&useractions.UserActionCfg{
			Tags:       []useractions.ActionTag{useractions.ActionTagMultiPaste},
			Attributes: map[string]string{useractions.AttributeInputField: string(inputField)},
		},
	); err != nil {
		s.Fatal("Fail to long press to select and delete item: ", err)
	}
}
