// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui2

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/local/chrome"
)

// RootDebugInfo returns the chrome.automation root as a string.
// If the JavaScript fails to execute, an error is returned.
func RootDebugInfo(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var out string
	err := tconn.EvalPromise(ctx, "tast.promisify(chrome.automation.getDesktop)().then(root => root+'');", &out)
	return out, err
}

// LogRootDebugInfo logs the chrome.automation root debug info to a file.
func LogRootDebugInfo(ctx context.Context, tconn *chrome.TestConn, filename string) error {
	debugInfo, err := RootDebugInfo(ctx, tconn)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(debugInfo), 0644)
}
