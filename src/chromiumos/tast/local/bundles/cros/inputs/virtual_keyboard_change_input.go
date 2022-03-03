// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardChangeInput,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that changing input method in different ways",
		Contacts:     []string{"shend@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name:              "tablet",
			Pre:               pre.VKEnabledTabletReset,
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "a11y",
			Pre:               pre.VKEnabledClamshellReset,
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "informational",
			ExtraAttr:         []string{"informational"},
			Pre:               pre.VKEnabledTabletReset,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}},
	})
}

func VirtualKeyboardChangeInput(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	inputMethod := ime.Japanese
	typingTestData, ok := data.TypingMessageHello.GetInputData(inputMethod)
	if !ok {
		s.Fatalf("Test Data for input method %v does not exist", inputMethod)
	}

	if err := inputMethod.Install(tconn)(ctx); err != nil {
		s.Fatalf("Failed to install input method %q: %v", inputMethod, err)
	}

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	ui := uiauto.New(tconn)
	vkbctx := vkb.NewContext(cr, tconn)

	inputField := testserver.TextAreaInputField
	inputMethodOption := vkb.NodeFinder.Name(inputMethod.Name).Role(role.StaticText)
	vkLanguageMenuFinder := vkb.KeyFinder.Name("open keyboard menu")

	validateAction := uiauto.Combine("verify changing input method on virtual keyboard",
		// Switch IME using virtual keyboard language menu.
		its.ClickFieldUntilVKShown(inputField),
		ui.LeftClick(vkLanguageMenuFinder),
		ui.LeftClick(inputMethodOption),
		ui.WaitUntilExists(vkb.NodeFinder.Name(inputMethod.ShortLabel).Role(role.StaticText)),

		// Validate current input method change.
		inputMethod.WaitUntilActivated(tconn),

		// Validate typing test.
		vkbctx.TapKeysIgnoringCase(typingTestData.CharacterKeySeq),
		vkbctx.SelectFromSuggestion(typingTestData.ExpectedText),
		util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), typingTestData.ExpectedText),
	)

	if err := uiauto.UserAction("Change input method on VK",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: fmt.Sprintf("Change input method to %q", inputMethod.Name),
			},
			Tags: []useractions.ActionTag{useractions.ActionTagSwitchIME},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to verify changing input method: ", err)
	}
}
