// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// InputEval creates a data sructure to define common input function and expected out.
type InputEval struct {
	TestName     string
	InputFunc    uiauto.Action
	ExpectedText string
}

// FieldInputEval creates a data sructure to define common input function and expected out on certain input field.
type FieldInputEval struct {
	InputField   string
	InputFunc    uiauto.Action
	ExpectedText string
}

// WaitForFieldTextToBe returns an action checking until the input field value equals given text.
func WaitForFieldTextToBe(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) uiauto.Action {
	ui := uiauto.New(tconn)
	return ui.Retry(10, func(ctx context.Context) error {
		nodeInfo, err := ui.Info(ctx, finder)
		if err != nil {
			return err
		} else if nodeInfo.Value != expectedText {
			return errors.Errorf("unexpected user name: got %s; want %s", nodeInfo.Value, expectedText)
		}
		return nil
	})
}
