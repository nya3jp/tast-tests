// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardMultitouch,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks typing on virtual keyboard with multiple simultaneous touches",
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture: fixture.TabletVKWithMultitouch,
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosTabletVKWithMultitouch,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func VirtualKeyboardMultitouch(ctx context.Context, s *testing.State) {
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

	vkbCtx := vkb.NewContext(cr, tconn)
	ui := uiauto.New(tconn)

	tsw, tcc, err := touch.NewTouchscreenAndConverter(ctx, tconn)
	if err != nil {
		s.Fatal("Fail to get touch screen: ", err)
	}
	defer tsw.Close()

	inputMethod := ime.EnglishUS
	if err := inputMethod.InstallAndActivateUserAction(uc)(ctx); err != nil {
		s.Fatal("Failed to set input method: ", err)
	}
	uc.SetAttribute(useractions.AttributeInputMethod, inputMethod.Name)

	inputField := testserver.TextAreaAutoShiftInSentence

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to get touch writer: ", err)
	}
	defer stw.Close()

	touchAndHold := func(finder *nodewith.Finder) uiauto.Action {
		return func(ctx context.Context) error {
			loc, err := ui.Location(ctx, finder)
			if err != nil {
				return errors.Wrapf(err, "failed to get the location of the node %v", finder)
			}
			x, y := tcc.ConvertLocation(loc.CenterPoint())
			if err := stw.Move(x, y); err != nil {
				return errors.Wrap(err, "failed to move the touch")
			}
			testing.Sleep(ctx, 50*time.Millisecond)
			return nil
		}
	}

	mousePressAndHold := func(finder *nodewith.Finder) uiauto.Action {
		return uiauto.Combine("mouse press on node center point and hold",
			ui.MouseMoveTo(finder, 10*time.Millisecond),
			mouse.Press(tconn, mouse.LeftButton),
			uiauto.Sleep(50*time.Millisecond),
		)
	}

	releaseTouch := func() uiauto.Action {
		return func(ctx context.Context) error {
			return stw.End()
		}
	}

	shiftKeyFinder := nodewith.Name("shift").Ancestor(vkb.NodeFinder.HasClass("key_pos_shift_left"))
	zKeyFinder := vkb.KeyByNameIgnoringCase("z")
	xKeyFinder := vkb.KeyByNameIgnoringCase("x")
	vKeyFinder := vkb.KeyByNameIgnoringCase("v")

	validateAction := uiauto.NamedCombine("Verify multitouch typing on VK",
		// Basic multitouch typing.
		its.ClickFieldUntilVKShown(inputField),
		touchAndHold(zKeyFinder),
		mousePressAndHold(xKeyFinder),
		// First character should be uppercase for autoshifted text field.
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), "Z"),
		releaseTouch(),
		touchAndHold(vKeyFinder),
		// VK should now be deshifted, so remaining characters should be lowercase.
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), "Zx"),
		mouse.Release(tconn, mouse.LeftButton),
		releaseTouch(),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), "Zxv"),

		// Holding shift while typing.
		touchAndHold(shiftKeyFinder),
		vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateShifted),
		vkbCtx.TapKeys(strings.Split("AB", "")),
		releaseTouch(),
		vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
		util.WaitForFieldTextToBe(tconn, inputField.Finder(), "ZxvAB"),
	)

	if err := uiauto.UserAction("Multitouch typing on virtual keyboard",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature:    useractions.FeatureVKTyping,
				useractions.AttributeInputField: string(inputField),
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to multitouch type on virtual keyboard: ", err)
	}
}
