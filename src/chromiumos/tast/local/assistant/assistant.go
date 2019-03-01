// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package assistant supports interaction with Assistant service.
package assistant

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// Enable brings up Google Assistant service and returns any errors.
func Enable(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		  chrome.autotestPrivate.setAssistantEnabled(true,
		    10 * 1000 /* timeout_ms */,
		    () => {
		      if (chrome.runtime.lastError === undefined) {
		        resolve();
		      } else {
		        reject(chrome.runtime.lastError.message);
		      }
		    });
		  })`, nil)
}
