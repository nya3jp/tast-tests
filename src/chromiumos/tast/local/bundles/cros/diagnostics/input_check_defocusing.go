// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	da "chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputCheckDefocusing,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Pressing and releasing keys won't affect key states when the input page isn't focused",
		Contacts: []string{
			"dpad@google.com",
			"jeff.lin@cienet.com",
			"xliu@cienet.com",
			"ashleydp@google.com",
			"zentaro@google.com",
			"cros-peripherals@google.com",
		},
		Fixture:      "diagnosticsPrepForInputDiagnostics",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalKeyboard()),
	})
}

func InputCheckDefocusing(ctx context.Context, s *testing.State) {
	cr, tconn := s.FixtValue().(*utils.FixtureData).Cr, s.FixtValue().(*utils.FixtureData).Tconn

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Lock diagnostics window to right side of the screen
	kb.AccelAction("alt+]")(ctx)

	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create chrome: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	ui := uiauto.New(tconn)

	// Lock chrome window to left side of the screen
	kb.AccelAction("alt+[")(ctx)

	// Finds the browser window and shifts focus to it
	focusBrowserWindow := func() action.Action {
		return func(ctx context.Context) error {
			window, err := ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool {
				return w.WindowType == ash.WindowTypeBrowser
			})

			if err != nil {
				return err
			}
			return window.ActivateWindow(ctx, tconn)
		}
	}

	mc := pointer.NewMouse(tconn)
	defer mc.Close()

	verifyKeyStateUnaffected := func(keyName string) action.Action {
		actionName := "verify " + keyName + " key states when input page isn't focused"
		return uiauto.NamedAction(actionName,
			uiauto.Combine(actionName,
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyNotPressed).First()),
				kb.AccelPressAction(keyName),
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyNotPressed).First()),
				kb.AccelReleaseAction(keyName),
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyNotPressed).First()),
			))
	}

	inputTab := da.DxInput.Ancestor(da.DxRootNode)
	if err := uiauto.Combine("verify pressing and releasing key won't affect key states",
		ui.LeftClick(inputTab),
		ui.LeftClick(da.DxInternalKeyboardTestButton),
		// Pressing and releasing an inoccuous key and check it's shown as tested.
		kb.AccelAction("x"),
		ui.WaitUntilExists(da.KeyNodeFinder("x", da.KeyTested).First()),
		// Switch focus to a different window and check a pops up message when losing the focus.
		focusBrowserWindow(),
		ui.WaitUntilExists(da.DxDefocusingMsg),
		// Pressing and releasing a few keys, each time checking keys are not reflected.
		verifyKeyStateUnaffected("shift"),
		verifyKeyStateUnaffected("1"),
		verifyKeyStateUnaffected("q"),
		// Switching focus back to the Diagnostics window and check pops up message is gone.
		kb.AccelAction("Alt+Tab"),
		ui.WaitUntilGone(da.DxDefocusingMsg),
		// Checking an inoccuous key still shown as tested.
		ui.WaitUntilExists(da.KeyNodeFinder("x", da.KeyTested).First()),
		// Checking keys are still marked as not pressed.
		ui.WaitUntilExists(da.KeyNodeFinder("shift", da.KeyNotPressed).First()),
		ui.WaitUntilExists(da.KeyNodeFinder("1", da.KeyNotPressed).First()),
		ui.WaitUntilExists(da.KeyNodeFinder("q", da.KeyNotPressed).First()),
		// Pressing another key and check it is reflected in the diagram.
		kb.AccelAction("ctrl"),
		ui.WaitUntilExists(da.KeyNodeFinder("ctrl", da.KeyTested).First()),
	)(ctx); err != nil {
		s.Fatal("Failed to check key states: ", err)
	}
}
