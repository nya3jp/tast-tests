// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardKioskTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that user can type on physical keyboard in kiosk mode",
		Contacts:     []string{"jhtin@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.DefaultInputMethod}),
		Timeout:      2 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:   fixture.KioskNonVK,
				ExtraAttr: []string{"informational", "group:input-tools-upstream"},
			},
			{
				Name:      "lacros",
				Fixture:   fixture.LacrosKioskNonVK,
				ExtraAttr: []string{"informational", "group:input-tools-upstream"},
			},
		},
	})
}

func PhysicalKeyboardKioskTyping(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	userContext, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), cr, tconn, nil)
	if err != nil {
		s.Fatal("Failed to create inputs user context: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	ui := uiauto.New(tconn)
	inputField := testserver.TextAreaInputField

	actionName := "PK find inputfield and type"
	if err := uiauto.UserAction(actionName,
		uiauto.Combine(actionName,
			ui.WaitUntilExists(inputField.Finder()),
			ui.MakeVisible(inputField.Finder()),
			ui.LeftClick(inputField.Finder()),
			keyboard.TypeAction("abcdefghijklmnopqrstuvwxyz01234! ABCDEFGHIJKLMNOPQRSTUVWXYZ01234!"),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), "abcdefghijklmnopqrstuvwxyz01234! ABCDEFGHIJKLMNOPQRSTUVWXYZ01234!"),
		),
		userContext,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeaturePKTyping,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate typing output: ", err)
	}
}
