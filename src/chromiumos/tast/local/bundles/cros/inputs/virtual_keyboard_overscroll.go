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

	// Get the most bottom text area of the page to ensure it is outside
	// the view port.
	lastInputField := testserver.TextAreaAutoShiftOff

	showVKByClickingAction := uiauto.Combine(
		"show VK by clicking on the last input field of the page",
		// Hide VK in the case that it is visible.
		vkbCtx.HideVirtualKeyboard(),
		// Click at the input field at the bottom of the page to
		// show VK.
		its.ClickFieldUntilVKShown(lastInputField),
	)

	waitForOverscrollCompletionAction := uiauto.Combine(
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
				return errors.Errorf(
					"VK bounds are empty: width = %d, height = %d",
					vkBounds.Width, vkBounds.Height)
			}

			// Get the input field bounds.
			inputFieldBounds, err := ui.Location(ctx, lastInputField.Finder())
			if err != nil {
				return errors.Wrap(err, "failed to get the input field bounds")
			}

			// Validate that overscroll is performed and the input field is
			// in the view and above VK.
			if inputFieldBounds.Top < 0 ||
				inputFieldBounds.Top > vkBounds.Top {
				return errors.Errorf(
					"the input field is overlaid by VK, "+
						"keyboardTop=%d,  inputFieldTop=%d",
					vkBounds.Top, inputFieldBounds.Top)
			}

			// Calc the difference between the bottom of the input field and
			// the top of the VK. The diff can be negative in the case of any
			// overlap.
			yDiff := vkBounds.Top -
				(inputFieldBounds.Top + inputFieldBounds.Height)

			// Validate that after overscroll, the input field is not too far
			// above the VK.
			if yDiff > vkBounds.Height {
				return errors.Errorf(
					"the input field is too far above VK, "+
						"keyboardTop=%d,  inputFieldTop=%d, "+
						"keyboardHeight=%d, inputFieldHeight=%d",
					vkBounds.Top, inputFieldBounds.Top,
					vkBounds.Height, inputFieldBounds.Height)
			}

			return nil
		}),
	)

	if err := uiauto.UserAction(
		"perform overscroll before showing VK",
		uiauto.Combine(
			"trigger VK to validate overscoll execution",
			// Click on the input field outside the viewport to enforce
			// overscroll.
			showVKByClickingAction,
			// Check that overscroll is performed.
			waitForOverscrollCompletionAction,
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField:   string(lastInputField),
				useractions.AttributeTestScenario: "Perform overscroll before showing VK",
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to overscroll: ", err)
	}

	if err := uiauto.UserAction(
		"trigger overscroll by typing using VK",
		uiauto.Combine(
			"trigger overscroll by typing on an input field outside the"+
				"view port",
			// Click on the input field outside the viewport to show VK and
			// set the caret in the input field.
			showVKByClickingAction,
			// Scroll to the top of the page to make the input field outside of
			// the viewport again while still keeping the caret inside it.
			its.ScrollTo(0, 0),
			// Wait for the view to be stable.
			uiauto.Sleep(200*time.Millisecond),
			// Type something in the input field that is not in the view port.
			vkbCtx.TapKeys([]string{"a", "b", "c"}),
			// Check that overscroll is performed to make the input field
			// visible.
			waitForOverscrollCompletionAction,
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeInputField:   string(lastInputField),
				useractions.AttributeTestScenario: "Perform overscroll when typing using VK",
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to overscroll: ", err)
	}
}
