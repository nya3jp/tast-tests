package uiauto

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/event"
)

// TODO(b/178020071): eventually migrate watchers over from the old ui package.
// Consider using go routines to deal with monitoring status and avoid sleeping.

// NewRootWatcher creates a new event watcher on the root node for the specified event type.
func NewRootWatcher(ctx context.Context, tconn *chrome.TestConn, event event.Event) (*ui.EventWatcher, error) {
	return ui.NewRootWatcher(ctx, tconn, ui.EventType(event))
}

// WaitForLocationChangeCompletedAction returns a uiauto.Action which calls ui.WaitForLocationChangeCompleted.
func WaitForLocationChangeCompletedAction(tconn *chrome.TestConn) Action {
	return NamedAction(
		"uiauto.WaitForLocationChangeCompletedAction(tconn *chrome.TestConn)",
		func(ctx context.Context) error { return ui.WaitForLocationChangeCompleted(ctx, tconn) })
}
