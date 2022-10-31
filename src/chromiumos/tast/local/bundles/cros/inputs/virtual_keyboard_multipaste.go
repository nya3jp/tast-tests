// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ime"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test multipaste virtual keyboard functionality",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		Params: []testing.Param{
			{
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
			},
			{
				Name:              "informational",
				Fixture:           fixture.TabletVK,
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels, hwdep.SkipOnPlatform("puff", "fizz")),
				ExtraAttr:         []string{"informational"},
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

func VirtualKeyboardMultipaste(ctx context.Context, s *testing.State) {
	const (
		text1        = "Hello world"
		text2        = "12345"
		expectedText = "Hello world12345"
	)

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext

	cleanupCtx := ctx
	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Launch inputs test web server.
	its, err := testserver.LaunchBrowser(ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

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

	if err := ash.SetClipboard(ctx, tconn, text1); err != nil {
		s.Fatal("Failed to set text1 to clipboard: ", err)
	}
	if err := ash.SetClipboard(ctx, tconn, text2); err != nil {
		s.Fatal("Failed to set text2 to clipboard: ", err)
	}

	actionName := "Input from VK multipaste clipboard"
	if err := uiauto.UserAction(
		actionName,
		uiauto.Combine("navigate to multipaste virtual keyboard and paste text",
			its.ClickFieldUntilVKShown(inputField),
			vkbCtx.SwitchToMultipaste(),
			uiauto.RetrySilently(3, uiauto.Combine("click on multipaste items",
				its.Clear(inputField),
				vkbCtx.TapMultipasteItem(text1),
				util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), text1),
			)),
			vkbCtx.TapMultipasteItem(text2),
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), expectedText),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField: string(inputField),
				useractions.AttributeFeature:    useractions.FeatureMultiPaste,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Fail to paste text through multipaste virtual keyboard: ", err)
	}

	actionName = "Select then de-select item in multipaste clipboard"
	ui := uiauto.New(tconn)

	if err := uiauto.UserAction(
		actionName,
		uiauto.Combine("Select then de-select item in multipaste virtual keyboard",
			touchCtx.LongPress(vkb.MultipasteItemFinder.Name(text1)),
			ui.WithTimeout(3*time.Second).WaitUntilExists(vkb.MultipasteTrashFinder),
			vkbCtx.TapMultipasteItem(text1),
			ui.WithTimeout(3*time.Second).WaitUntilGone(vkb.MultipasteTrashFinder),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField: string(inputField),
				useractions.AttributeFeature:    useractions.FeatureMultiPaste,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Fail to select then de-select item: ", err)
	}

	actionName = "Remove item in VK multipaste clipboard"
	if err := uiauto.UserAction(
		actionName,
		vkbCtx.DeleteMultipasteItem(touchCtx, text1),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeatureMultiPaste,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Fail to long press to select and delete item: ", err)
	}
}
