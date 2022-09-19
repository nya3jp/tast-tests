// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
)

// TestEnv encapsulates resources useful in many clipboard history tests.
type TestEnv struct {
	Tconn *chrome.TestConn
	UI    *uiauto.Context
	Kb    *input.KeyboardEventWriter
	Br    *browser.Browser
	Cb    uiauto.Action
}

const clipboardHistoryTextItemViewClassName = "ClipboardHistoryTextItemView"

// FindFirstTextItem returns a finder which locates the first text item in the
// clipboard history menu.
func FindFirstTextItem() *nodewith.Finder {
	return nodewith.ClassName(clipboardHistoryTextItemViewClassName).First()
}

// SetUpEnv returns an initialized TestEnv or error on initialization failure.
func SetUpEnv(ctx context.Context, cr *chrome.Chrome, bt browser.Type) (TestEnv, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return TestEnv{}, errors.Wrap(err, "failed to connect to test API")
	}

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return TestEnv{}, errors.Wrap(err, "failed to get keyboard")
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		return TestEnv{}, errors.Wrap(err, "failed to open the browser")
	}

	return TestEnv{tconn, ui, kb, br, closeBrowser}, nil
}
