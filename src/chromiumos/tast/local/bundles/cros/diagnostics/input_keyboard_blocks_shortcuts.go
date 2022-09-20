// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"

	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome/uiauto"
	da "chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	la "chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputKeyboardBlocksShortcuts,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Keyboard input tester blocks all shortcuts only when open",
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

func InputKeyboardBlocksShortcuts(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*utils.FixtureData).Tconn

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	launcher := nodewith.ClassName(la.ExpandedItemsClass).Visible().First()
	diagnosticsApp := da.DxRootNode
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Verify Shortcuts are blocked in keyboard tester window",
		ui.LeftClick(da.DxInput.Ancestor(diagnosticsApp)),
		ui.LeftClick(da.DxInternalKeyboardTestButton),
		// Control + w should not close the window
		kb.AccelAction("ctrl+w"),
		ui.WaitUntilExists(diagnosticsApp),
		// Try to open launcher and verify it does not open
		kb.AccelAction("search"),
		ui.Gone(launcher),
		// Closes keyboard tester window
		kb.AccelAction("alt+esc"),
		// Verify launcher can be launched now
		kb.AccelAction("search"),
		ui.WaitUntilExists(launcher),
		kb.AccelAction("search"),
		ui.WaitUntilGone(launcher),
		// Verify ctrl + w will close the diagnostics window
		kb.AccelAction("ctrl+w"),
		ui.WaitUntilGone(diagnosticsApp),
	)(ctx); err != nil {
		s.Fatal("Failed to check key states: ", err)
	}
}
