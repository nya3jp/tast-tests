// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardCapsLock,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that user can lock Caps on physical keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.DefaultInputMethod}),
		Timeout:      2 * time.Minute,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				Fixture:           fixture.ClamshellNonVK,
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "informational",
				Fixture:           fixture.ClamshellNonVK,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosClamshellNonVK,
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func PhysicalKeyboardCapsLock(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	ui := uiauto.New(tconn)
	capsOnImageFinder := nodewith.Name("CAPS LOCK is on").Role(role.Image)

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	inputField := testserver.TextAreaInputField

	defer its.CloseAll(cleanupCtx)

	subtests := []struct {
		name     string
		scenario string
		action   uiauto.Action
	}{
		// Type and check that the text field has the correct Hiragana.
		{
			name:     "CapsLockImageDisplayed",
			scenario: "Should display caps lock is on image",
			action: uiauto.Combine("pk caps lock and unlock",
				keyboard.AccelAction("Alt+Search"),
				ui.WaitUntilExists(capsOnImageFinder),
				keyboard.AccelAction("Shift"),
				ui.WaitUntilGone(capsOnImageFinder),
			),
		},
		{
			name:     "TypesInBrowserInCapsLock",
			scenario: "Types in browser in all caps.",
			action: uiauto.Combine("pk caps lock, type and do a case sensitive match",
				keyboard.AccelAction("Alt+Search"),
				its.Clear(inputField),
				its.ClickFieldAndWaitForActive(inputField),
				keyboard.TypeAction("abcdefghijklmnopqrstuvwxyz01234! ABCDEFGHIJKLMNOPQRSTUVWXYZ01234!"),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), "ABCDEFGHIJKLMNOPQRSTUVWXYZ01234! abcdefghijklmnopqrstuvwxyz01234!"),
			),
		},
	}

	for _, subtest := range subtests {
		s.Run(ctx, subtest.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+string(subtest.name))

			if err := uiauto.UserAction(
				"PK CapsLock",
				subtest.action,
				uc,
				&useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeTestScenario: subtest.scenario,
						useractions.AttributeFeature:      useractions.FeaturePKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatal("Failed to validate caps lock: ", err)
			}
		})
	}
}
