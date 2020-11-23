// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

// WaitUntilExistsStatus repeatedly checks the existence of a node
// until the desired status is found or the timeout is reached.
// If the JavaScript fails to execute, an error is returned.
func WaitUntilExistsStatus(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, exists bool, timeout time.Duration) error {
	if exists {
		return ui.WaitUntilExists(ctx, tconn, params, timeout)
	}

	return ui.WaitUntilGone(ctx, tconn, params, timeout)
}
