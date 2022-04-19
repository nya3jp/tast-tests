// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	da "chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputCheckDefocusing,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Pressing and releasing keys won't affect key states when the input page isn't focused",
		Contacts:     []string{"jeff.lin@cienet.com", "xliu@cienet.com", "cros-peripherals@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func InputCheckDefocusing(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.Region("us"), chrome.EnableFeatures("DiagnosticsAppNavigation", "EnableInputInDiagnosticsApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create chrome: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	dxRootNode, err := da.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch diagnostics app: ", err)
	}
	defer da.Close(cleanupCtx, tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
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

	inputTab := da.DxInput.Ancestor(dxRootNode)
	if err := uiauto.Combine("verify pressing and releasing key won't affect key states",
		ui.LeftClick(inputTab),
		ui.LeftClick(da.DxInternalKeyboardTestButton),
		// Pressing and releasing an inoccuous key and check it's shown as tested.
		kb.AccelAction("x"),
		ui.WaitUntilExists(da.KeyNodeFinder("x", da.KeyTested).First()),
		// Switch focus to a different window and check a pops up message when losing the focus.
		kb.AccelAction("Alt+Tab"),
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
