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
		Func:         InputKeyboardConnectAndDisconnect,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Connect a virtual keyboard on the diagnostics input page and than disconnect it",
		Contacts:     []string{"jeff.lin@cienet.com", "xliu@cienet.com", "cros-peripherals@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

func InputKeyboardConnectAndDisconnect(ctx context.Context, s *testing.State) {
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

	ui := uiauto.New(tconn)
	inputTab := da.DxInput.Ancestor(dxRootnode)
	virtualKeyboard := da.DxVirtualKeyboardHeading
	if err := uiauto.Combine("check no virtual keyboard exists in input device list",
		ui.LeftClick(inputTab),
		ui.Gone(virtualKeyboard),
	)(ctx); err != nil {
		s.Fatal("Failed to check virtual keyboard: ", err)
	}

	vkb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a virtual keyboard: ", err)
	}
	defer vkb.Close()

	disconnectKeyboard := func() action.Action {
		return func(ctx context.Context) error {
			return vkb.Close()
		}
	}

	if err := uiauto.Combine("verify virtual keyboard appears and disappears in the device list",
		ui.WaitUntilExists(virtualKeyboard),
		disconnectKeyboard(),
		ui.WaitUntilGone(virtualKeyboard),
	)(ctx); err != nil {
		s.Fatal("Failed to execute keyboard test: ", err)
	}
}
