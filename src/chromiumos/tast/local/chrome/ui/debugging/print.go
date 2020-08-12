// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package debugging provides helper functions for debugging test failures.
package debugging

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// PrintUITree prints ui tree into console log.
func PrintUITree(ctx context.Context, tconn *chrome.TestConn) {
	uiTree, err := ui.RootDebugInfo(ctx, tconn)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get ui tree: ", err)
	}
	testing.ContextLog(ctx, uiTree)
}

// PrintChromeTargets prints all available Chrome targets into console log.
func PrintChromeTargets(ctx context.Context, cr *chrome.Chrome) {
	allTargetInfo, err := cr.FindAllTargets(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to find targets info: ", err)
	}
	for _, targetInfo := range allTargetInfo {
		testing.ContextLogf(ctx, "%+v", targetInfo)
	}

}
