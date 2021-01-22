// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sharesheet supports controlling the Sharesheet on Chrome OS.
package sharesheet

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// ClickApp clicks the requested app on the sharesheet.
// The app must be available in the first 8 apps.
func ClickApp(ctx context.Context, tconn *chrome.TestConn, appShareLabel string) error {
	pollOpts := testing.PollOptions{Timeout: 15 * time.Second}

	// Get the app button to click.
	params := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: appShareLabel,
	}

	return ui.StableFindAndClick(ctx, tconn, params, &pollOpts)
}
