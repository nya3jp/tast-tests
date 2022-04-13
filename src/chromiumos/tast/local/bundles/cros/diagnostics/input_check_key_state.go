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
		Func:         InputCheckKeyState,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify keys states after pressed and released in the diagonstics input page",
		Contacts:     []string{"jeff.lin@cienet.com", "xliu@cienet.com", "cros-peripherals@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

func InputCheckKeyState(ctx context.Context, s *testing.State) {
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

	dxRootnode, err := da.Launch(ctx, tconn)
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
	inputTab := da.DxInput.Ancestor(dxRootnode)
	verifyKeyState := func(keyName string) action.Action {
		actionName := "verify " + keyName + " key states right after press and release"
		return uiauto.NamedAction(actionName,
			uiauto.Combine(actionName,
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyNotPressed).First()),
				kb.AccelPressAction(keyName),
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyPressed).First()),
				kb.AccelReleaseAction(keyName),
				ui.WaitUntilExists(da.KeyNodeFinder(keyName, da.KeyTested).First()),
			))
	}

	if err := uiauto.Combine("verify keys' states after pressed and released",
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
