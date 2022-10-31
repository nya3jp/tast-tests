// Copyright 2022 The ChromiumOS Authors
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
		Contacts:     []string{"essential-inputs-gardener-oncall@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.EnglishUS}),
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:      "tablet",
				Fixture:   fixture.TabletVK,
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:      "clamshell",
				Fixture:   fixture.ClamshellVK,
				ExtraAttr: []string{"group:input-tools-upstream"},
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosTabletVK,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
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
	// showing VK without performing overscroll will occlude the input field.
	inputField := testserver.OffscreenTextField

	showVKByClickingAction := uiauto.NamedCombine(
		"show VK by clicking on the last input field of the page",
		// Scroll into the input field and align its bottom with the bottom of
		// the view port.
		its.ScrollIntoView(inputField, false),
		// Click on the input field and wait for the VK to show up.
		vkbCtx.ClickUntilVKShown(inputField.Finder()),
	)

	// The function checks whether the visibility of the input field matches
	// a given expectation assuming that the VK is already visible.
	validateInputFieldVisibility := func(ctx context.Context, expectedVisibility bool) error {
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

		// Check if the input field is inside the screen and not occluded
		// by VK.
		visible := inputFieldBounds.Top > 0 && inputFieldBounds.Top < vkBounds.Top

		// Return error if the visibility of the input field mismatch the
		// expectation.
		if visible != expectedVisibility {
			expectedVisibilityStr := "offscreen"
			if expectedVisibility {
				expectedVisibilityStr = "visible"
			}

			return errors.Errorf("the input field must be %s, keyboardTop=%d, inputFieldTop=%d", expectedVisibilityStr, vkBounds.Top, inputFieldBounds.Top)
		}

		return nil
	}

	waitForInputFieldVisibility := func(expectedVisibility bool) uiauto.Action {
		return uiauto.RetrySilently(3, func(ctx context.Context) error {
			return validateInputFieldVisibility(ctx, expectedVisibility)
		})
	}

	if err := uiauto.UserAction(
		"perform overscroll before showing VK",
		uiauto.NamedCombine(
			"trigger VK to validate overscoll execution",
			// Scroll into the input field (currently is outside the view port)
			// and align its bottom with the bottom of the view port, then
			// click on it to trigger VK.
			showVKByClickingAction,
			// Allow sometimes for overscroll to be performed.
			uiauto.Sleep(200*time.Millisecond),
			// Check that overscroll is performed and the input field is not
			// occluded.
			waitForInputFieldVisibility(true),
			// Hide VK to cleanup the state.
			vkbCtx.HideVirtualKeyboard(),
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
			waitForInputFieldVisibility(true),
			// Hide VK to cleanup the state.
			vkbCtx.HideVirtualKeyboard(),
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
