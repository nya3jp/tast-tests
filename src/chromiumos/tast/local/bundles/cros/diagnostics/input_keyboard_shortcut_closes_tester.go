// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"

	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome/uiauto"
	da "chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputKeyboardShortcutClosesTester,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Keyboard input tester window closes with keyboard shortcut",
		Contacts: []string{
			"dpad@google.com",
			"cros-peripherals@google.com",
		},
		Fixture:      "diagnosticsPrepForInputDiagnostics",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalKeyboard()),
	})
}

func InputKeyboardShortcutClosesTester(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*utils.FixtureData).Tconn

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Verify Alt+Esc keyboard shortcut closes keyboard tester",
		ui.LeftClick(da.DxInput.Ancestor(da.DxRootNode)),
		ui.LeftClick(da.DxInternalKeyboardTestButton),
		ui.WaitUntilExists(da.KeyNodeFinder("a", da.KeyNotPressed).First()),
		kb.AccelAction("a"),
		ui.WaitUntilExists(da.KeyNodeFinder("a", da.KeyTested).First()),
		// Closes keyboard tester window
		kb.AccelAction("alt+esc"),
		ui.WaitUntilGone(da.KeyNodeFinder("a", da.KeyTested).First()),
	)(ctx); err != nil {
		s.Fatal("Failed to check key states: ", err)
	}
}
