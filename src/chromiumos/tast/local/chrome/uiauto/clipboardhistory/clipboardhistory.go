// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type TestEnv struct {
	Tconn *chrome.TestConn
	Ui    *uiauto.Context
	Kb    *input.KeyboardEventWriter
	Br    *browser.Browser
}

const clipboardHistoryTextItemViewClassName = "ClipboardHistoryTextItemView"

// FindFirstTextItem returns a finder which locates the first text item in the
// clipboard history menu.
func FindFirstTextItem() *nodewith.Finder {
	return nodewith.ClassName(clipboardHistoryTextItemViewClassName).First()
}

func SetUpEnv(ctx context.Context, s *testing.State, cr *chrome.Chrome, bt browser.Type) TestEnv {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	return TestEnv{Tconn: tconn, Ui: ui, Kb: kb, Br: br}
}
