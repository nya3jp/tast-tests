// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardEnglishSettings,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that the input settings works in Chrome",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Pre:               pre.VKEnabledTablet,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Pre:               pre.VKEnabledTablet,
				Name:              "informational",
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "fixture",
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func VirtualKeyboardEnglishSettings(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome
	var tconn *chrome.TestConn
	var uc *useractions.UserContext
	if strings.Contains(s.TestName(), "fixture") {
		cr = s.FixtValue().(fixture.FixtData).Chrome
		tconn = s.FixtValue().(fixture.FixtData).TestAPIConn
		uc = s.FixtValue().(fixture.FixtData).UserContext
		uc.SetTestName(s.TestName())
	} else {
		cr = s.PreValue().(pre.PreData).Chrome
		tconn = s.PreValue().(pre.PreData).TestAPIConn
		uc = s.PreValue().(pre.PreData).UserContext
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Revert settings to default after testing.
	defer func() {
		if err := tconn.Eval(cleanupCtx, `chrome.inputMethodPrivate.setSettings(
			"xkb:us::eng",
			{"virtualKeyboardEnableCapitalization": true,
			"virtualKeyboardAutoCorrectionLevel": 1})`, nil); err != nil {
			s.Log("Failed to revert language settings")
		}
	}()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Use text input field as testing target.
	inputField := testserver.TextInputField

	type testData struct {
		name                  string
		capitalizationEnabled bool
		keySeq                []string
		expectedText          string
		ime                   ime.InputMethod
	}

	subTests := []testData{
		{
			name:                  "capitalizationEnabled",
			capitalizationEnabled: true,
			keySeq:                strings.Split("Hello", ""),
			expectedText:          "Hello",
			ime:                   ime.EnglishUS,
		}, {
			name:                  "capitalizationDisabled",
			capitalizationEnabled: false,
			keySeq:                strings.Split("hello", ""),
			expectedText:          "hello",
			ime:                   ime.EnglishUS,
		},
	}

	for _, subTest := range subTests {
		s.Run(ctx, subTest.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+subTest.name)
			if !subTest.capitalizationEnabled {
				if err := imesettings.SetVKAutoCapitalization(uc, subTest.ime, subTest.capitalizationEnabled)(ctx); err != nil {
					s.Fatal("Failed to change IME settings: ", err)
				}
			}

			vkbCtx := vkb.NewContext(cr, tconn)

			validateAction := uiauto.Combine("verify VK input",
				vkbCtx.WaitForDecoderEnabled(true),
				its.Clear(inputField),
				its.ClickFieldUntilVKShown(inputField),
				vkbCtx.TapKeys(subTest.keySeq),
				util.WaitForFieldTextToBe(tconn, inputField.Finder(), subTest.expectedText),
			)

			testScenario := "VK typing with auto-cap disabled"
			if subTest.capitalizationEnabled {
				testScenario = "VK typing with auto-cap enabled"
			}

			if err := uiauto.UserAction(
				"VK typing input",
				validateAction,
				uc,
				&useractions.UserActionCfg{
					Attributes: map[string]string{
						useractions.AttributeInputField:   string(inputField),
						useractions.AttributeTestScenario: testScenario,
						useractions.AttributeFeature:      useractions.FeatureAutoCapitalization,
					},
				},
			)(ctx); err != nil {
				s.Fatal("Failed to verify input: ", err)
			}
		})
	}
}
