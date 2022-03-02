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
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardMultipasteSuggestion,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test multipaste suggestion functionality",
		Contacts:     []string{"jiwan@chromium.org", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		Pre:          pre.VKEnabledTabletWithMultipasteSuggestion,
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "informational",
			ExtraAttr:         []string{"informational"},
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels, hwdep.SkipOnPlatform("puff", "fizz")),
		}},
	})
}

func VirtualKeyboardMultipasteSuggestion(ctx context.Context, s *testing.State) {
	const (
		text = "Hello world"
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

	// Select the input field being tested.
	inputField := testserver.TextAreaInputField
	vkbCtx := vkb.NewContext(cr, tconn)
	touchCtx, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Fail to get touch screen: ", err)
	}
	defer touchCtx.Close()

	ash.SetClipboard(ctx, tconn, text)

	actionName := "Input text through multipaste suggestion bar"
	if err := uiauto.UserAction(
		actionName,
		uiauto.Combine(actionName,
			its.ClickFieldUntilVKShown(inputField),
			vkbCtx.TapMultipasteSuggestion(text),
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), text),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField: string(inputField),
				useractions.AttributeFeature:    useractions.FeatureMultiPaste,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Fail to paste text through multipaste suggestion: ", err)
	}
}
