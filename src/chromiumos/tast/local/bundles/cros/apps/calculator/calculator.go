// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package calc contains common functions used in the Calculator app.
package calc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
)

// UIConn returns a connection to the Calculator app HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The caller should close the returned connection. e.g. defer calcConn.Close().
func UIConn(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	// Establish a Chrome connection to the Calculator app and wait for it to finish loading.
	targetURL := "https://calculator.apps.chrome/"
	appConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(targetURL))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to target %q", targetURL)
	}
	if err := appConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "failed to wait for Calculator app to finish loading")
	}
	return appConn, nil
}

// TapKey taps key on the app by executing click function on located web element.
// keyName is aria-label of the element, not displayed text. e.g. Do not use '+' but 'plus'.
func TapKey(appConn *chrome.Conn, keyName string) uiauto.Action {
	script := fmt.Sprintf(`document.querySelector(".keypad canvas[aria-role='button'][aria-label='%s']").click()`, keyName)
	return func(ctx context.Context) error {
		if err := appConn.Eval(ctx, script, nil); err != nil {
			return errors.Wrapf(err, "failed to tap key %q", keyName)
		}
		return nil
	}
}

// WaitForCalculateResult waits until the calculation result is expected.
func WaitForCalculateResult(appConn *chrome.Conn, expectedResult string) uiauto.Action {
	script := `document.querySelector(".calculator-display").innerText`
	var result string

	return action.Retry(3, func(ctx context.Context) error {
		if err := appConn.Eval(ctx, script, &result); err != nil {
			return errors.Wrap(err, "failed to get calculation result")
		}
		if result != expectedResult {
			return errors.Errorf("Wrong calculation result: got %q; want %q", result, expectedResult)
		}
		return nil
	}, time.Second)
}
