// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
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

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAccent,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that long pressing keys pop up accent window",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.FrenchFrance}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "informational",
				Fixture:           fixture.TabletVK,
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
		},
	})
}

func VirtualKeyboardAccent(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	ui := uiauto.New(tconn)

	inputMethod := ime.FrenchFrance
	const (
		keyName       = "e"
		accentKeyName = "é"
		languageLabel = "FR"
	)

	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatal("Failed to set input method: ", err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	inputField := testserver.TextAreaNoCorrectionInputField
	accentContainerFinder := nodewith.HasClass("accent-container")
	accentKeyFinder := nodewith.Ancestor(accentContainerFinder).Name(accentKeyName).Role(role.StaticText)
	languageLabelFinder := vkb.NodeFinder.Name(languageLabel).First()
	keyFinder := vkb.KeyByNameIgnoringCase(keyName)

	validateAction := uiauto.Combine("input accent letter with virtual keyboard",
		its.ClickFieldUntilVKShown(inputField),
		ui.WaitUntilExists(languageLabelFinder),
		ui.MouseMoveTo(keyFinder, 500*time.Millisecond),
		mouse.Press(tconn, mouse.LeftButton),
		// Popup accent window sometimes flash on showing, so using Retry instead of WaitUntilExist.
		ui.WithInterval(time.Second).RetrySilently(10, ui.WaitForLocation(accentContainerFinder)),
		ui.MouseMoveTo(accentKeyFinder, 500*time.Millisecond),
		mouse.Release(tconn, mouse.LeftButton),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), accentKeyName),
	)

	if err := uiauto.UserAction("VK typing accent letters",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: fmt.Sprintf(`long press %q to trigger accent popup then select %q`, keyName, accentKeyName),
				useractions.AttributeFeature:      useractions.FeatureVKTyping,
				useractions.AttributeInputField:   string(inputField),
			},
		},
	)(ctx); err != nil {
		s.Fatal("Fail to input accent key on virtual keyboard: ", err)
	}
}
