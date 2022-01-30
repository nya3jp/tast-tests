// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browserutil implements convenience utils needed to take actions with ash-chrome, lacros-chrome or browser instances.
// TODO: can't move this to browser package due to cyclic imports.
package browserutil

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

// CloseAboutBlank finds a target that is about:blank, closes one (likely the last opened), then waits until it's gone.
func CloseAboutBlank(ctx context.Context, br *browser.Browser, bt browser.Type) error {
	// TODO: can br have bt returned?
	// TODO: can br keep a tconn to Ash TestAPIConn? how to make sure it's valid at the time it's called?
	if bt == browser.TypeLacros {
		targets, err := br.FindTargets(ctx, chrome.MatchTargetURL(chrome.BlankURL))
		if err != nil {
			testing.ContextLog(ctx, "Failed to query for about:blank pages on lacros-chrome, err: ", err)
		}
		if len(targets) > 0 {
			err := br.CloseTarget(ctx, targets[0].TargetID)
			if err != nil {
				testing.ContextLog(ctx, "Failed to close lacros-chrome window, err: ", err)
			}
		}
	}
	return nil
}

func isBrowserWindow(w *ash.Window, bt browser.Type, titlePrefix string) bool {
	switch bt {
	case browser.TypeAsh:
		titlePrefix = "Chrome - " + titlePrefix
		return w.WindowType == ash.WindowTypeBrowser && strings.HasPrefix(w.Title, titlePrefix)
	case browser.TypeLacros:
		return w.WindowType == ash.WindowTypeLacros && strings.HasPrefix(w.Title, titlePrefix)
	}
	panic("unreachable")
}

// WaitForWindow takes *ash-chrome*'s TestConn as tconn, not the one provided by Lacros.TestAPIConn.
func WaitForWindow(ctx context.Context, tconn *chrome.TestConn, bt browser.Type, titlePrefix string) error {
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsVisible && isBrowserWindow(w, bt, titlePrefix)
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for visible browser window (titlePrefix: %v)", titlePrefix)
	}
	return nil
}
