// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingOmnibox,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that the virtual keyboard works in Chrome browser omnibox",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Pre:               pre.VKEnabledTabletReset,
			Val:               []ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.ChinesePinyin},
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "guest",
			Pre:               pre.VKEnabledTabletInGuest,
			Val:               []ime.InputMethod{ime.EnglishUSWithInternationalKeyboard, ime.JapaneseWithUSKeyboard, ime.Korean},
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "a11y",
			Pre:               pre.VKEnabledClamshellReset,
			Val:               []ime.InputMethod{ime.EnglishUK, ime.AlphanumericWithJapaneseKeyboard, ime.Arabic},
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "informational",
			Pre:               pre.VKEnabledTabletReset,
			Val:               []ime.InputMethod{ime.EnglishUS, ime.Japanese, ime.Arabic},
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			ExtraAttr:         []string{"informational"},
		}}})
}

func VirtualKeyboardTypingOmnibox(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext
	vkbCtx := vkb.NewContext(cr, tconn)

	testIMEs := s.Param().([]ime.InputMethod)

	subtest := func(testName string, inputData data.InputData) func(ctx context.Context, s *testing.State) {
		return func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			// Use a shortened context for test operations to reserve time for cleanup.
			ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer shortCancel()

			defer func(ctx context.Context) {
				outDir := filepath.Join(s.OutDir(), testName)
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+testName)

				if err := vkbCtx.HideVirtualKeyboard()(ctx); err != nil {
					s.Log("Failed to hide virtual keyboard: ", err)
				}
			}(cleanupCtx)

			// Warning: Please do not launch Browser via cr.NewConn(ctx, "")
			// to test omnibox typing. It might be indeterminate whether default url string
			// "about:blank" is highlighted or not.
			// In that case, typing test can either replace existing url or insert into it.
			// A better way to do it is launching Browser from launcher, url is empty by default.
			if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
				s.Fatalf("Failed to launch %s: %s", apps.Chrome.Name, err)
			}
			defer func(ctx context.Context) {
				if err := apps.Close(ctx, tconn, apps.Chrome.ID); err != nil {
					testing.ContextLog(ctx, "Failed to close Chrome browser: ", err)
				}
			}(cleanupCtx)

			if err := ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute); err != nil {
				s.Fatalf("%s did not appear in shelf after launch: %s", apps.Chrome.Name, err)
			}

			omniboxFinder := nodewith.Role(role.TextField).Attribute("inputType", "url")

			validateAction := uiauto.Combine("verify virtual keyboard input on omnibox",
				vkbCtx.ClickUntilVKShown(omniboxFinder),
				vkbCtx.TapKeysIgnoringCase(inputData.CharacterKeySeq),
				func(ctx context.Context) error {
					if inputData.SubmitFromSuggestion {
						return vkbCtx.SelectFromSuggestion(inputData.ExpectedText)(ctx)
					}
					return nil
				},
				// Validate text.
				util.WaitForFieldTextToBeIgnoringCase(tconn, omniboxFinder, inputData.ExpectedText),
			)

			if err := uiauto.UserAction("VK typing",
				validateAction,
				uc,
				&useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeInputField: "Omnibox",
						useractions.AttributeFeature:    useractions.FeatureVKTyping,
					},
				},
			)(ctx); err != nil {
				s.Fatal("Failed to verify virtual keyboard input on omnibox: ", err)
			}
		}
	}
	util.RunSubtestsPerInputMethodAndMessage(ctx, uc, s, testIMEs, []data.Message{data.TypingMessageHello}, subtest)
}
