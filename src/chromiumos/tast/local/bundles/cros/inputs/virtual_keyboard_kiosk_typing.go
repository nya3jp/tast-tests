// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardKioskTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that user can type in virtual keyboard in kiosk mode",
		Contacts:     []string{"jhtin@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.DefaultInputMethod}),
		Timeout:      2 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:   fixture.KioskVK,
				ExtraAttr: []string{"informational"},
			},
			{
				Name:      "lacros",
				Fixture:   fixture.LacrosKioskVK,
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func VirtualKeyboardKioskTyping(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(fixture.InputsKioskFixtData).TestAPIConn
	uc := s.FixtValue().(fixture.InputsKioskFixtData).UserContext

	vkbCtx := vkb.NewContext(cr, tconn)
	defer vkbCtx.HideVirtualKeyboard()(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	ui := uiauto.New(tconn)
	inputField := testserver.TextAreaInputField

	actionName := "VK typing in inputfield in Kiosk mode"
	if err := uiauto.UserAction(actionName,
		uiauto.Combine(actionName,
			ui.WaitUntilExists(inputField.Finder()),
			ui.MakeVisible(inputField.Finder()),
			vkbCtx.ClickUntilVKShown(inputField.Finder()),
			vkbCtx.TapKeysIgnoringCase(strings.Split("abcdefghijklmnopqrstuvwxyz", "")),
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "abcdefghijklmnopqrstuvwxyz"),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeatureVKTyping,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate VK typing: ", err)
	}
}
