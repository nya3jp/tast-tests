// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"strings"
	"time"

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
		Func:         InputCheckRegionalKey,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Input page shows expected regional keyboard layout with different region code",
		Contacts:     []string{"jeff.lin@cienet.com", "xliu@cienet.com", "cros-peripherals@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "us",
		}, {
			Name: "jp",
		}, {
			Name: "fr",
		}},
	})
}

func InputCheckRegionalKey(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	regionCode := strings.Split(s.TestName(), "InputCheckRegionalKey.")[1]
	cr, err := chrome.New(ctx, chrome.Region(regionCode), chrome.EnableFeatures("DiagnosticsAppNavigation", "EnableInputInDiagnosticsApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	dxRootnode, err := da.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch diagnostics app: ", err)
	}
	defer da.Close(cleanupCtx, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	internalKeyboardTestButton, ok := da.DxInternalKeyboardTestButtons[regionCode]
	if !ok {
		s.Fatalf("Region code %v has not defined yet: ", regionCode)
	}
	ui := uiauto.New(tconn)
	inputTab := da.DxInput.Ancestor(dxRootnode)
	if err := uiauto.Combine("checks regional keys for region "+regionCode+" keyboard layout",
		ui.LeftClick(inputTab),
		ui.LeftClick(internalKeyboardTestButton),
		da.CheckGlyphsbyRegion(ui, regionCode),
	)(ctx); err != nil {
		s.Fatal("Failed to checks regional keys: ", err)
	}
}
