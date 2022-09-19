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
		Func:         InputCheckKeyState,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Input page shows expected key states when keys are pressed and released",
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

func InputCheckKeyState(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*utils.FixtureData).Tconn

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
	verifyKeyState := func(keyName string) action.Action {
		actionName := "verify " + keyName + " key states right after pressing and releasing the key"
		return uiauto.NamedAction(actionName,
			uiauto.Combine(actionName,
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyNotPressed).First()),
				kb.AccelPressAction(keyName),
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyPressed).First()),
				kb.AccelReleaseAction(keyName),
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyTested).First()),
			))
	}

	inputTab := da.DxInput.Ancestor(da.DxRootNode)
	if err := uiauto.Combine("verify the specific keys' states after pressing and releasing the key",
		ui.LeftClick(inputTab),
		ui.LeftClick(da.DxInternalKeyboardTestButton),
		verifyKeyState("backspace"),
		verifyKeyState("tab"),
		verifyKeyState("shift"),
		verifyKeyState("q"),
		verifyKeyState("1"),
	)(ctx); err != nil {
		s.Fatal("Failed to check key states: ", err)
	}
}
