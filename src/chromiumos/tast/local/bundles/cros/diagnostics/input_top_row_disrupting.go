// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome/uiauto"
	da "chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputTopRowDisrupting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Pressing several disruptive keys won't disrupt the test and affect other keys' states",
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

func InputTopRowDisrupting(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*utils.FixtureData).Tconn

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	ui := uiauto.New(tconn)
	inoccuousKey := "x"
	clickDisruptiveKey := func(topRowKey, keyNodeName string) action.Action {
		actionName := "verify disruptive key " + topRowKey + " doesn't disrupt the test"
		return uiauto.NamedAction(actionName, uiauto.Combine(actionName,
			ui.WaitUntilExists(da.KeyNodeFinder(keyNodeName, da.KeyNotPressed).First()),
			kb.AccelPressAction(topRowKey),
			ui.WaitUntilExists(da.KeyNodeFinder(keyNodeName, da.KeyPressed).First()),
			ui.WaitUntilExists(da.DxKeyboardTester),
			ui.WaitUntilExists(da.KeyNodeFinder(inoccuousKey, da.KeyTested).First()),
			kb.AccelReleaseAction(topRowKey),
			ui.WaitUntilExists(da.KeyNodeFinder(keyNodeName, da.KeyTested).First()),
		))
	}

	inputTab := da.DxInput.Ancestor(da.DxRootNode)
	if err := uiauto.Combine("verify disruptive keys don't disrupt the test and won't affect other key state",
		ui.LeftClick(inputTab),
		ui.LeftClick(da.DxInternalKeyboardTestButton),
		// Pressing and releasing an inoccuous key and check it's shown as pressed in the diagram.
		kb.AccelAction(inoccuousKey),
		ui.WaitUntilExists(da.KeyNodeFinder(inoccuousKey, da.KeyTested).First()),
		// Clicking disruptive keys than check tester is still visible and the inoccuous key still shown as tested.
		clickDisruptiveKey(topRow.BrowserBack, "Back"),
		clickDisruptiveKey(topRow.BrowserRefresh, "Refresh"),
		clickDisruptiveKey(topRow.ZoomToggle, "Fullscreen"),
		clickDisruptiveKey(topRow.SelectTask, "Overview"),
		clickDisruptiveKey(topRow.Screenshot, "Screenshot"),
		clickDisruptiveKey(topRow.BrightnessDown, "Display brightness down"),
		clickDisruptiveKey(topRow.BrightnessUp, "Display brightness up"),
		clickDisruptiveKey(topRow.VolumeMute, "Mute"),
		clickDisruptiveKey(topRow.VolumeDown, "Volume down"),
		clickDisruptiveKey(topRow.VolumeUp, "Volume up"),
	)(ctx); err != nil {
		s.Fatal("Failed to test disruptive keys: ", err)
	}
}
