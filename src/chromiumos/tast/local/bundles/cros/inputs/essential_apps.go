// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/googleapps"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// testCase struct encapsulates parameters for each test.
type testCase struct {
	feature        string
	inputMethod    ime.InputMethod
	keyName        string
	typingKeys     string
	expectedResult string
	languageLabel  string
}

// const ()

func init() {
	testing.AddTest(&testing.Test{
		Func:         EssentialApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "VK in Google doc",
		Contacts:     []string{"xiuwen@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "french_vk_accentkey",
				Fixture:           fixture.GoogleDocs,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: testCase{
					feature:        "accent_key",
					inputMethod:    ime.FrenchFrance,
					keyName:        "e",
					expectedResult: "é",
					languageLabel:  "FR",
				},
			},
			{
				Name:              "frech_vk_typing",
				Fixture:           fixture.GoogleDocs,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: testCase{
					feature:        "typing",
					inputMethod:    ime.FrenchFrance,
					typingKeys:     "word",
					expectedResult: "word",
					languageLabel:  "FR",
				},
			},
			{
				Name:              "frech_vk_typing_slide",
				Fixture:           fixture.GoogleSlides,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				Val: testCase{
					feature:        "typing",
					inputMethod:    ime.FrenchFrance,
					typingKeys:     "word",
					expectedResult: "word",
					languageLabel:  "FR",
				},
			},
		},
	})
}

func EssentialApps(ctx context.Context, s *testing.State) {
	testCase := s.Param().(testCase)
	cr := s.FixtValue().(fixture.AppFixtData).Chrome
	tconn := s.FixtValue().(fixture.AppFixtData).TestAPIConn
	uc := s.FixtValue().(fixture.AppFixtData).UserContext
	vkbCtx := s.FixtValue().(fixture.AppFixtData).VirtualKeyboardContext
	conn := s.FixtValue().(fixture.AppFixtData).ChromeConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	inputMethod := testCase.inputMethod

	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatal("Failed to set input method: ", err)
	}

	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	if err := vkbCtx.ShowVirtualKeyboard()(ctx); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	ui := uiauto.New(tconn)
	languageLabelFinder := vkb.NodeFinder.Name(testCase.languageLabel).First()

	switch testCase.feature {
	case "accent_key":

		accentKeyName := testCase.expectedResult
		keyName := testCase.keyName

		accentContainerFinder := nodewith.HasClass("accent-container")
		accentKeyFinder := nodewith.Ancestor(accentContainerFinder).Name(accentKeyName).Role(role.StaticText)
		keyFinder := vkb.KeyByNameIgnoringCase(keyName)

		validateAction := uiauto.Combine("input accent letter with virtual keyboard",
			ui.WaitUntilExists(languageLabelFinder),
			ui.MouseMoveTo(keyFinder, 500*time.Millisecond),
			mouse.Press(tconn, mouse.LeftButton),
			// Popup accent window sometimes flash on showing, so using Retry instead of WaitUntilExist.
			ui.WithInterval(time.Second).RetrySilently(10, ui.WaitForLocation(accentContainerFinder)),
			ui.MouseMoveTo(accentKeyFinder, 500*time.Millisecond),
			mouse.Release(tconn, mouse.LeftButton),
			util.VerifyTextShownFromScreenshot(tconn, vkbCtx, accentKeyName, true),
			googleapps.WaitUntilDocContentToBe(conn, accentKeyName),
		)

		if err := uiauto.UserAction("VK typing accent letters",
			validateAction,
			uc,
			&useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeTestScenario: fmt.Sprintf(`long press %q to trigger accent popup then select %q`, keyName, accentKeyName),
					useractions.AttributeFeature:      useractions.FeatureVKTyping,
				},
			},
		)(ctx); err != nil {
			s.Fatal("Fail to input accent key on virtual keyboard: ", err)
		}

	case "typing":
		validateAction := uiauto.Combine("type letters with virtual keyboard",
			ui.WaitUntilExists(languageLabelFinder),
			vkbCtx.TapKeys(strings.Split(testCase.typingKeys, "")),
			util.VerifyTextShownFromScreenshot(tconn, vkbCtx, testCase.expectedResult, false),
			googleapps.WaitUntilDocContentToBe(conn, testCase.expectedResult),
		)

		if err := uiauto.UserAction("VK typing letters",
			validateAction,
			uc,
			&useractions.UserActionCfg{
				Attributes: map[string]string{
					useractions.AttributeTestScenario: fmt.Sprintf(`"typing word %q`, testCase.typingKeys),
					useractions.AttributeFeature:      useractions.FeatureVKTyping,
				},
			},
		)(ctx); err != nil {
			s.Fatal("Fail to type key on virtual keyboard: ", err)
		}

	}

}
