// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/fixture"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardOverscroll,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check that overscroll is performed correctly when showing VK",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Params: []testing.Param{
			{
				Name:      "tablet",
				ExtraAttr: []string{"informational"},
				Fixture:   fixture.TabletVK,
			},
			{
				Name:      "clamshell",
				ExtraAttr: []string{"informational"},
				Fixture:   fixture.ClamshellVK,
			},
			{
				Name:              "lacros",
				ExtraAttr:         []string{"informational"},
				Fixture:           fixture.LacrosTabletVK,
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func VirtualKeyboardOverscroll(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	uc := s.FixtValue().(fixture.FixtData).UserContext
	ui := uiauto.New(tconn)

	// Create a context for the virtual keyboard.
	vkbCtx := vkb.NewContext(cr, tconn)

	// Launch the test server.
	its, err := testserver.LaunchBrowser(
		ctx, s.FixtValue().(fixture.FixtData).BrowserType, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.CloseAll(cleanupCtx)

	// Get a text input field outside the current view port so that an scroll
	// can align its bottom with the bottom of the view port. After that,
	// showing VK without performing overscroll will occlude the
	// input field.
	inputField := testserver.TextAreaAutoShiftOff

	showVKByClickingAction := uiauto.NamedCombine(
		"show VK by clicking on the last input field of the page",
		// Hide VK in the case that it is visible.
		vkbCtx.HideVirtualKeyboard(),
		// Scroll into the input field and align its bottom with the bottom of
		// the view port.
		its.ScrollIntoView(inputField, false),
		// Click on the input field and wait for the VK to show up.
		vkbCtx.ClickUntilVKShown(inputField.Finder()),
	)

	waitForOverscrollCompletionAction := uiauto.NamedCombine(
		"validate the input field is not overlaid by VK",
		// Allow 200 milliseconds for the overscroll to be performed.
		uiauto.Sleep(200*time.Millisecond),
		// Check if overscroll is performed by comparing the location
		// of the input field and the VK bounds.
		uiauto.RetrySilently(3, func(ctx context.Context) error {
			// Get the VK bounds.
			vkBounds, err := vkbCtx.Location(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get VK bounds")
			}

			// Ensure VK is shown and the bounds are set.
			if vkBounds.Width == 0 || vkBounds.Height == 0 {
				return errors.Errorf("VK bounds are empty: width = %d, height = %d", vkBounds.Width, vkBounds.Height)
			}

			// Get the input field bounds.
			inputFieldBounds, err := ui.Location(ctx, inputField.Finder())
			if err != nil {
				return errors.Wrap(err, "failed to get the input field bounds")
			}

			// Validate that overscroll is performed and the input field is
			// in the view and its top is strictly above the top of the VK.
			if inputFieldBounds.Top <= 0 || inputFieldBounds.Top >= vkBounds.Top {
				return errors.Errorf("the input field is overlaid by VK, keyboardTop=%d, inputFieldTop=%d", vkBounds.Top, inputFieldBounds.Top)
			}

			return nil
		}),
	)

	if err := uiauto.UserAction(
		"perform overscroll before showing VK",
		uiauto.NamedCombine(
			"trigger VK to validate overscoll execution",
			// Scroll into the input field (currently is outside the view port)
			// and align its bottom with the bottom of the view port, then
			// click on it to trigger VK.
			showVKByClickingAction,
			// Check that overscroll is performed and the input field is not
			// occluded.
			waitForOverscrollCompletionAction,
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField:   string(inputField),
				useractions.AttributeTestScenario: "Perform overscroll before showing VK",
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to overscroll: ", err)
	}

	if err := uiauto.UserAction(
		"trigger overscroll by typing using VK",
		uiauto.NamedCombine(
			"trigger overscroll by typing on an input field outside the view port",
			// Scroll into the input field (currently is outside the view port)
			// to align its bottom with the bottom of the view port, then
			// click on it to trigger VK.
			showVKByClickingAction,
			// Scroll to the top of the page to make the input field outside of
			// the view port again while still keeping the caret inside it.
			its.ScrollTo(0, 0),
			// Wait for the view to be stable.
			uiauto.Sleep(200*time.Millisecond),
			// Type something in the input field that is not in the view port.
			vkbCtx.TapKeysIgnoringCase([]string{"a", "b", "c"}),
			// Check that overscroll is performed to make the input field
			// visible.
			waitForOverscrollCompletionAction,
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField:   string(inputField),
				useractions.AttributeTestScenario: "Perform overscroll when typing using VK",
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to overscroll: ", err)
	}
}
