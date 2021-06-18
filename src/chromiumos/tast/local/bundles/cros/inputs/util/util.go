// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// InputEval is a data structure to define common input function and expected out.
type InputEval struct {
	TestName     string
	InputFunc    uiauto.Action
	ExpectedText string
}

// WaitForFieldTextToBe returns an action checking whether the input field value equals given text.
// The text is case sensitive.
func WaitForFieldTextToBe(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) uiauto.Action {
	return waitForFieldTextFunc(tconn, finder, expectedText, false)
}

// WaitForFieldTextToBeIgnoringCase returns an action checking whether the input field value equals given text.
// The text is case insensitive.
func WaitForFieldTextToBeIgnoringCase(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) uiauto.Action {
	return waitForFieldTextFunc(tconn, finder, expectedText, true)
}

// waitForFieldTextFunc returns an action checking whether the input field value equals given text.
// The text can either be case sensitive or not.
func waitForFieldTextFunc(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string, ignoreCase bool) uiauto.Action {
	ui := uiauto.New(tconn).WithInterval(time.Second)
	return uiauto.Combine("validate field text",
		// Sleep 200ms before validating text field.
		// Without sleep, it almost never pass the first time check due to the input delay.
		ui.Sleep(200*time.Millisecond),
		ui.Retry(5, func(ctx context.Context) error {
			nodeInfo, err := ui.Info(ctx, finder)
			if err != nil {
				return err
			} else if !ignoreCase && nodeInfo.Value != expectedText {
				return errors.Errorf("failed to validate input value: got: %s; want: %s", nodeInfo.Value, expectedText)
			} else if ignoreCase && strings.ToLower(nodeInfo.Value) != strings.ToLower(expectedText) {
				return errors.Errorf("failed to validate input value ignoring case: got: %s; want: %s", nodeInfo.Value, expectedText)
			}
			return nil
		}))
}
