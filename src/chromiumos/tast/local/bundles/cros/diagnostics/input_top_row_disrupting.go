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
		Func:         InputTopRowDisrupting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Pressing several disruptive keys won't disrupt the test and affect other keys' states",
		Contacts: []string{
			"jeff.lin@cienet.com",
			"xliu@cienet.com",
			"dpad@google.com",
			"ashleydp@google.com",
			"zentaro@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func InputTopRowDisrupting(ctx context.Context, s *testing.State) {
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
	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	ui := uiauto.New(tconn)
	inoccuousKey := "x"
	clickDisruptiveKey := func(topRowKey, keyNodeName string) action.Action {
		actionName := "verify disruptive key " + topRowKey + " don't disrupt the test"
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

	inputTab := da.DxInput.Ancestor(dxRootNode)
	if err := uiauto.Combine("verify disruptive keys don't disrupt the test and won't affect other key state",
		ui.LeftClick(inputTab),
		ui.LeftClick(da.DxInternalKeyboardTestButton),
		// Pressing and releasing an inoccuous key and check it's shown as pressed in the diagram.
		kb.AccelAction(inoccuousKey),
		ui.WaitUntilExists(da.KeyNodeFinder(inoccuousKey, da.KeyTested).First()),
		// Clicking disruptive keys than check tester is still visible and the inoccuous key still shown as tested.
		clickDisruptiveKey(topRow.BrowserBack, "Back"),
		clickDisruptiveKey(topRow.ZoomToggle, "Fullscreen"),
		clickDisruptiveKey(topRow.BrightnessDown, "Display brightness down"),
		clickDisruptiveKey(topRow.BrightnessUp, "Display brightness up"),
	)(ctx); err != nil {
		s.Fatal("Failed to test disruptive keys: ", err)
	}
}
