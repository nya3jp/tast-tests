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

var pollOptions = &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}

// WaitForWindow waits for a browser window to be visible whose title equals or
// extends the given titlePrefix.
// Args: tconn is a TestConn from *ash-chrome*.
func WaitForWindow(ctx context.Context, tconn *chrome.TestConn, bt browser.Type, titlePrefix string) error {
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsVisible && isBrowserWindow(w, bt, titlePrefix)
	}, pollOptions); err != nil {
		return errors.Wrapf(err, "failed to wait for visible %v browser window (titlePrefix: %v)", bt, titlePrefix)
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
	return false // an unknown window. code should not reach here
}
